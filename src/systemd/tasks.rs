use std::collections::HashMap;
use std::fmt::Write as _;
use std::path::PathBuf;

use serde::Serialize;

use crate::error::{Error, Result};
use crate::systemd::constants::SERVICE_PREFIX;
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::abs_command_path;
use crate::systemd::types::Service;

#[derive(Debug, Clone)]
pub struct TaskSchedule {
    pub on_calendar: String,
    pub on_boot_sec: String,
    pub on_startup_sec: String,
    pub on_unit_active_sec: String,
    pub on_unit_inactive_sec: String,
    pub persistent: bool,
    pub randomized_delay_sec: String,
    pub accuracy_sec: String,
}

#[derive(Debug, Clone)]
pub struct Task {
    pub name: String,
    pub description: String,
    pub command: String,
    pub args: Vec<String>,
    pub workdir: String,
    pub environment: HashMap<String, String>,
    pub schedule: TaskSchedule,
}

#[derive(Debug, Clone, Serialize)]
pub struct TaskInfo {
    pub service: String,
    pub task: String,
    pub mode: SystemdMode,
    pub next: String,
    pub last: String,
    pub timer_unit: String,
    pub activates: String,
}

pub fn task_service_unit(service_name: &str, task_name: &str) -> String {
    format!("{SERVICE_PREFIX}{service_name}-task-{task_name}.service")
}

pub fn task_timer_unit(service_name: &str, task_name: &str) -> String {
    format!("{SERVICE_PREFIX}{service_name}-task-{task_name}.timer")
}

pub fn task_env_file_name(service_name: &str, task_name: &str) -> String {
    format!("{service_name}-task-{task_name}.env")
}

pub fn task_env_file_path(
    service_name: &str,
    task_name: &str,
    mode: SystemdMode,
) -> Result<PathBuf> {
    let name = task_env_file_name(service_name, task_name);
    match mode {
        SystemdMode::System => Ok(PathBuf::from(format!("/etc/smdctl/env/{name}"))),
        SystemdMode::User => Ok(home_dir()?.join(".config/smdctl/env").join(&name)),
    }
}

/// Per-task log file (user mode, file-based logging).
/// Returns `None` for system mode (logs go to journald) or when HOME is unset.
pub fn task_log_file_path(
    service_name: &str,
    task_name: &str,
    mode: SystemdMode,
) -> Option<PathBuf> {
    match mode {
        SystemdMode::System => None,
        SystemdMode::User => std::env::var("HOME").ok().map(|h| {
            PathBuf::from(h)
                .join(".config/smdctl/logs")
                .join(format!("{service_name}-task-{task_name}.log"))
        }),
    }
}

pub fn task_service_path(
    service_name: &str,
    task_name: &str,
    mode: SystemdMode,
) -> Result<PathBuf> {
    unit_path_disk(&task_service_unit(service_name, task_name), mode)
}

pub fn task_timer_path(service_name: &str, task_name: &str, mode: SystemdMode) -> Result<PathBuf> {
    unit_path_disk(&task_timer_unit(service_name, task_name), mode)
}

fn home_dir() -> Result<PathBuf> {
    std::env::var("HOME")
        .map(PathBuf::from)
        .map_err(|_| Error::msg("HOME environment variable is not set"))
}

fn unit_path_disk(unit_file: &str, mode: SystemdMode) -> Result<PathBuf> {
    match mode {
        SystemdMode::System => Ok(PathBuf::from("/etc/systemd/system").join(unit_file)),
        SystemdMode::User => Ok(home_dir()?.join(".config/systemd/user").join(unit_file)),
    }
}

pub fn generate_task_service_file(parent: &Service, task: &Task) -> Result<String> {
    let mut sb = String::new();
    sb.push_str("[Unit]\n");
    if !task.description.is_empty() {
        let _ = writeln!(sb, "Description={}", task.description);
    } else {
        let _ = writeln!(sb, "Description=smdctl task: {}/{}", parent.name, task.name);
    }
    if !parent.after.is_empty() {
        let _ = writeln!(sb, "After={}", parent.after.join(" "));
    } else if matches!(parent.mode, SystemdMode::User) {
        sb.push_str("After=default.target\n");
    } else {
        sb.push_str("After=network-online.target\n");
    }
    if !parent.wants.is_empty() {
        let _ = writeln!(sb, "Wants={}", parent.wants.join(" "));
    }
    sb.push('\n');

    sb.push_str("[Service]\n");
    sb.push_str("Type=oneshot\n");

    let command = abs_command_path(&task.command, &task.workdir);
    let exec_start = if task.args.is_empty() {
        command.clone()
    } else {
        format!("{} {}", command, task.args.join(" "))
    };
    let _ = writeln!(sb, "ExecStart={exec_start}");

    if !task.workdir.is_empty() {
        let _ = writeln!(sb, "WorkingDirectory={}", task.workdir);
    }
    if matches!(parent.mode, SystemdMode::System) && !parent.user.is_empty() {
        let _ = writeln!(sb, "User={}", parent.user);
    }

    let envp = task_env_file_path(&parent.name, &task.name, parent.mode)?;
    let _ = writeln!(sb, "EnvironmentFile=-{}", envp.to_string_lossy());

    if let Some(lf) = task_log_file_path(&parent.name, &task.name, parent.mode) {
        let lp = lf.to_string_lossy();
        let _ = writeln!(sb, "StandardOutput=append:{lp}");
        let _ = writeln!(sb, "StandardError=append:{lp}");
    }
    sb.push('\n');
    Ok(sb)
}

pub fn generate_task_timer_file(
    service_name: &str,
    task_name: &str,
    schedule: &TaskSchedule,
) -> String {
    let mut sb = String::new();
    sb.push_str("[Unit]\n");
    let _ = writeln!(sb, "Description=smdctl timer: {service_name}/{task_name}\n");
    sb.push_str("[Timer]\n");
    if !schedule.on_calendar.is_empty() {
        let _ = writeln!(sb, "OnCalendar={}", schedule.on_calendar);
    }
    if !schedule.on_boot_sec.is_empty() {
        let _ = writeln!(sb, "OnBootSec={}", schedule.on_boot_sec);
    }
    if !schedule.on_startup_sec.is_empty() {
        let _ = writeln!(sb, "OnStartupSec={}", schedule.on_startup_sec);
    }
    if !schedule.on_unit_active_sec.is_empty() {
        let _ = writeln!(sb, "OnUnitActiveSec={}", schedule.on_unit_active_sec);
    }
    if !schedule.on_unit_inactive_sec.is_empty() {
        let _ = writeln!(sb, "OnUnitInactiveSec={}", schedule.on_unit_inactive_sec);
    }

    let _ = writeln!(sb, "Unit={}", task_service_unit(service_name, task_name));
    let _ = writeln!(sb, "Persistent={}", schedule.persistent);
    if !schedule.randomized_delay_sec.is_empty() {
        let _ = writeln!(sb, "RandomizedDelaySec={}", schedule.randomized_delay_sec);
    }
    if !schedule.accuracy_sec.is_empty() {
        let _ = writeln!(sb, "AccuracySec={}", schedule.accuracy_sec);
    }
    sb.push('\n');
    sb.push_str("[Install]\n");
    sb.push_str("WantedBy=timers.target\n");
    sb
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn timer_file_contains_calendar_and_install() {
        let sched = TaskSchedule {
            on_calendar: "daily".into(),
            on_boot_sec: "".into(),
            on_startup_sec: "".into(),
            on_unit_active_sec: "".into(),
            on_unit_inactive_sec: "".into(),
            persistent: true,
            randomized_delay_sec: "".into(),
            accuracy_sec: "".into(),
        };
        let content = generate_task_timer_file("svc", "cleanup", &sched);
        assert!(content.contains("OnCalendar=daily\n"));
        assert!(content.contains("WantedBy=timers.target\n"));
        assert!(content.contains("Unit=smdctl-svc-task-cleanup.service\n"));
    }

    #[test]
    fn oneshot_user_logging() {
        let parent = Service {
            mode: SystemdMode::User,
            name: "svc".into(),
            description: "".into(),
            command: "".into(),
            args: vec![],
            workdir: "/opt/svc".into(),
            user: "".into(),
            environment: HashMap::new(),
            restart: "always".into(),
            timeout_start: 90,
            timeout_stop: 30,
            kill_mode: "control-group".into(),
            after: vec![],
            wants: vec![],
            private_tmp: false,
            protect_system: "".into(),
            no_new_privileges: false,
            limit_nofile: 0,
            tasks_max: 0,
        };
        let task = Task {
            name: "cleanup".into(),
            description: "".into(),
            command: "/bin/true".into(),
            args: vec![],
            workdir: "/opt/svc".into(),
            environment: HashMap::new(),
            schedule: TaskSchedule {
                on_calendar: "daily".into(),
                on_boot_sec: "".into(),
                on_startup_sec: "".into(),
                on_unit_active_sec: "".into(),
                on_unit_inactive_sec: "".into(),
                persistent: true,
                randomized_delay_sec: "".into(),
                accuracy_sec: "".into(),
            },
        };
        std::env::set_var("HOME", "/tmp/smdctl-test-home");
        let content = generate_task_service_file(&parent, &task).expect("gen");
        assert!(content.contains("Type=oneshot\n"));
        assert!(content.contains("StandardOutput=append:"));
    }

    #[test]
    fn timer_file_emits_all_schedule_fields() {
        let sched = TaskSchedule {
            on_calendar: "hourly".into(),
            on_boot_sec: "5min".into(),
            on_startup_sec: "1min".into(),
            on_unit_active_sec: "6h".into(),
            on_unit_inactive_sec: "1h".into(),
            persistent: false,
            randomized_delay_sec: "30s".into(),
            accuracy_sec: "1s".into(),
        };
        let content = generate_task_timer_file("svc", "t", &sched);
        for needle in [
            "OnCalendar=hourly\n",
            "OnBootSec=5min\n",
            "OnStartupSec=1min\n",
            "OnUnitActiveSec=6h\n",
            "OnUnitInactiveSec=1h\n",
            "Persistent=false\n",
            "RandomizedDelaySec=30s\n",
            "AccuracySec=1s\n",
            "WantedBy=timers.target\n",
        ] {
            assert!(content.contains(needle), "missing {needle:?}\n{content}");
        }
    }

    #[test]
    fn oneshot_system_mode_has_no_log_directives() {
        let parent = Service {
            mode: SystemdMode::System,
            name: "svc".into(),
            description: "".into(),
            command: "".into(),
            args: vec![],
            workdir: "/opt/svc".into(),
            user: "root".into(),
            environment: HashMap::new(),
            restart: "always".into(),
            timeout_start: 90,
            timeout_stop: 30,
            kill_mode: "control-group".into(),
            after: vec![],
            wants: vec![],
            private_tmp: false,
            protect_system: "".into(),
            no_new_privileges: false,
            limit_nofile: 0,
            tasks_max: 0,
        };
        let task = Task {
            name: "cleanup".into(),
            description: "".into(),
            command: "/bin/true".into(),
            args: vec![],
            workdir: "/opt/svc".into(),
            environment: HashMap::new(),
            schedule: TaskSchedule {
                on_calendar: "daily".into(),
                on_boot_sec: "".into(),
                on_startup_sec: "".into(),
                on_unit_active_sec: "".into(),
                on_unit_inactive_sec: "".into(),
                persistent: true,
                randomized_delay_sec: "".into(),
                accuracy_sec: "".into(),
            },
        };
        let content = generate_task_service_file(&parent, &task).expect("gen");
        assert!(!content.contains("StandardOutput=append:"));
        assert!(content.contains("User=root\n"));
        assert!(content.contains("EnvironmentFile=-/etc/smdctl/env/svc-task-cleanup.env"));
    }
}
