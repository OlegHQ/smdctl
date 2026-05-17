//! Scheduled-task operations: create / list (data structures live in `tasks`).

use std::collections::HashMap;
use std::fs;
use std::path::Path;

use chrono::{Local, TimeZone, Utc};

use crate::error::{Error, Result};
use crate::systemd::constants::SERVICE_PREFIX;
use crate::systemd::manager::{validate_service_name, Manager};
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::get_log_dir;
use crate::systemd::tasks::{
    generate_task_service_file, generate_task_timer_file, task_env_file_path, task_service_path,
    task_timer_path, task_timer_unit, Task, TaskInfo,
};
use crate::systemd::types::Service;

impl Manager {
    /// List timers via `systemctl list-timers --output=json` and filter to
    /// task units owned by smdctl.
    pub fn list_tasks(&self, all: bool, service_filter: &str) -> Result<Vec<TaskInfo>> {
        let pattern = if service_filter.is_empty() {
            "smdctl-*-task-*.timer".to_string()
        } else {
            format!("{SERVICE_PREFIX}{service_filter}-task-*.timer")
        };

        // `&[&str]` requires owned-string-derived slices; collect once.
        let mut owned: Vec<&str> =
            vec!["list-timers", "--output=json", "--no-pager", "--no-legend"];
        if all {
            owned.push("--all");
        }
        owned.push(&pattern);

        let output = self.exec.output(self.mode, &owned)?;

        let rows: Vec<serde_json::Value> = serde_json::from_str(&output)?;
        let mut tasks = Vec::new();
        for row in rows {
            let unit = row["unit"].as_str().unwrap_or("").to_string();
            if !unit.starts_with(SERVICE_PREFIX) || !unit.ends_with(".timer") {
                continue;
            }
            let base = unit.trim_end_matches(".timer");
            let trimmed = base.strip_prefix(SERVICE_PREFIX).unwrap_or(base);
            let parts: Vec<&str> = trimmed.splitn(2, "-task-").collect();
            if parts.len() != 2 {
                continue;
            }
            let svc_name = parts[0].to_string();
            let task_name = parts[1].to_string();
            if !service_filter.is_empty() && svc_name != service_filter {
                continue;
            }
            let next = row["next"]
                .as_i64()
                .or_else(|| row["next"].as_f64().map(|f| f as i64));
            let last = row["last"]
                .as_i64()
                .or_else(|| row["last"].as_f64().map(|f| f as i64));
            let activates = row["activates"].as_str().unwrap_or("").to_string();

            tasks.push(TaskInfo {
                service: svc_name,
                task: task_name,
                mode: self.mode,
                next: format_list_timer_ts(next),
                last: format_list_timer_ts(last),
                timer_unit: unit,
                activates,
            });
        }
        Ok(tasks)
    }

    pub fn create_task(&self, parent: &Service, task: &Task) -> Result<()> {
        validate_service_name(&parent.name)?;
        validate_service_name(&task.name)?;
        if parent.mode != self.mode {
            return Err(Error::msg(format!(
                "parent service mode {:?} does not match manager mode {:?}",
                parent.mode, self.mode
            )));
        }
        if task.command.is_empty() {
            return Err(Error::msg(format!(
                "task command is required: {}/{}",
                parent.name, task.name
            )));
        }
        let s = &task.schedule;
        if s.on_calendar.is_empty()
            && s.on_boot_sec.is_empty()
            && s.on_startup_sec.is_empty()
            && s.on_unit_active_sec.is_empty()
            && s.on_unit_inactive_sec.is_empty()
        {
            return Err(Error::msg(format!(
                "task schedule is required: {}/{}",
                parent.name, task.name
            )));
        }

        let svc_path = task_service_path(&parent.name, &task.name, parent.mode)?;
        if let Some(dir) = svc_path.parent() {
            fs::create_dir_all(dir)?;
        }

        let env_path = task_env_file_path(&parent.name, &task.name, parent.mode)?;
        if let Some(dir) = env_path.parent() {
            fs::create_dir_all(dir)?;
        }
        if matches!(parent.mode, SystemdMode::User) {
            if let Ok(log_dir) = get_log_dir(parent.mode) {
                let _ = fs::create_dir_all(log_dir);
            }
        }

        let one_shot = generate_task_service_file(parent, task)?;
        fs::write(&svc_path, one_shot)?;

        let timer_content = generate_task_timer_file(&parent.name, &task.name, &task.schedule);
        let tpath = task_timer_path(&parent.name, &task.name, parent.mode)?;
        fs::write(&tpath, timer_content)?;

        write_env_file(
            &env_path,
            &format!("{}/{}", parent.name, task.name),
            &task.environment,
        )?;

        self.daemon_reload()?;

        let timer_unit = task_timer_unit(&parent.name, &task.name);
        self.enable_unit(&timer_unit)?;
        self.start_unit(&timer_unit)?;
        Ok(())
    }
}

fn write_env_file(path: &Path, label: &str, env: &HashMap<String, String>) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    let mut content = String::new();
    content.push_str(&format!(
        "# Environment variables for smdctl: {label}\n# Edit this file with your editor\n\n"
    ));
    for (k, v) in env {
        content.push_str(&format!("{k}={v}\n"));
    }
    fs::write(path, content)?;
    Ok(())
}

fn format_list_timer_ts(v: Option<i64>) -> String {
    let Some(ts) = v else {
        return "-".to_string();
    };
    if ts == 0 {
        return "-".to_string();
    }
    let secs = ts.div_euclid(1_000_000);
    let micros = ts.rem_euclid(1_000_000);
    let nsecs = (micros * 1000) as u32;
    match Utc.timestamp_opt(secs, nsecs) {
        chrono::LocalResult::Single(dt) => dt
            .with_timezone(&Local)
            .format("%Y-%m-%d %H:%M")
            .to_string(),
        _ => "-".to_string(),
    }
}
