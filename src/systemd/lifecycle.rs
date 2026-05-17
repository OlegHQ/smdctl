//! Service lifecycle: create / remove / start / stop / restart / enable / disable.

use std::collections::HashMap;
use std::fs;
use std::path::Path;

use crate::error::{Error, Result};
use crate::systemd::constants::SERVICE_PREFIX;
use crate::systemd::generation::generate_service_file;
use crate::systemd::manager::{validate_service_name, Manager};
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::{
    env_file_path, get_config_dir, get_log_dir, log_file_path, service_path, systemd_unit_name,
};
use crate::systemd::tasks::{task_env_file_path, task_log_file_path};
use crate::systemd::types::Service;

impl Manager {
    pub fn create(&self, svc: &Service) -> Result<()> {
        validate_service_name(&svc.name)?;
        if svc.mode != self.mode {
            return Err(Error::msg(format!(
                "service mode {:?} does not match manager mode {:?}",
                svc.mode, self.mode
            )));
        }
        if self.service_exists(&svc.name) {
            return Err(Error::ServiceAlreadyExists(svc.name.clone()));
        }

        let content = generate_service_file(svc)?;
        let path = service_path(&svc.name, svc.mode)?;
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }

        if matches!(svc.mode, SystemdMode::User) {
            if let Ok(log_dir) = get_log_dir(svc.mode) {
                let _ = fs::create_dir_all(log_dir);
            }
        }

        fs::write(&path, content)?;

        if !svc.environment.is_empty() {
            self.create_env_file(&svc.name, &svc.environment)
                .map_err(|e| Error::msg(format!("create environment file: {e}")))?;
        }

        self.daemon_reload()?;
        Ok(())
    }

    pub fn create_env_file(&self, service_name: &str, env: &HashMap<String, String>) -> Result<()> {
        let env_dir = get_config_dir(self.mode)?;
        fs::create_dir_all(&env_dir)?;
        let mut content = String::new();
        content.push_str(&format!(
            "# Environment variables for smdctl service: {service_name}\n"
        ));
        content.push_str(&format!(
            "# Edit this file with: smdctl env {service_name}\n\n"
        ));
        for (k, v) in env {
            content.push_str(&format!("{k}={v}\n"));
        }
        let env_path = env_dir.join(format!("{service_name}.env"));
        fs::write(env_path, content)?;
        Ok(())
    }

    pub fn service_exists(&self, name: &str) -> bool {
        if service_path(name, self.mode)
            .map(|p| p.exists())
            .unwrap_or(false)
        {
            return true;
        }
        let other = match self.mode {
            SystemdMode::User => SystemdMode::System,
            SystemdMode::System => SystemdMode::User,
        };
        service_path(name, other)
            .map(|p| p.exists())
            .unwrap_or(false)
    }

    pub fn start(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        self.start_unit(&unit)
    }

    pub fn stop(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        self.stop_unit(&unit)
    }

    pub fn restart(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        self.exec.run(self.mode, &["restart", &unit])
    }

    pub fn enable(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        self.enable_unit(&unit)
    }

    pub fn disable(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        self.disable_unit(&unit)
    }

    pub fn remove(&self, name: &str) -> Result<()> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let _ = self.remove_tasks(name);
        let _ = self.stop(name);
        let _ = self.disable(name);

        let path = service_path(name, self.mode)?;
        fs::remove_file(path)?;

        if let Ok(envp) = env_file_path(name, self.mode) {
            let _ = fs::remove_file(envp);
        }

        if let Some(lp) = log_file_path(name, self.mode) {
            let _ = fs::remove_file(lp);
        }

        self.daemon_reload()?;
        Ok(())
    }

    pub fn remove_tasks(&self, service_name: &str) -> Result<()> {
        let svc_path = service_path(service_name, self.mode)?;
        let unit_dir = svc_path.parent().map(Path::to_path_buf).unwrap_or_default();
        let prefix = format!("{SERVICE_PREFIX}{service_name}-task-");

        let entries = match fs::read_dir(&unit_dir) {
            Ok(e) => e,
            Err(_) => return Ok(()),
        };

        for entry in entries.flatten() {
            let path = entry.path();
            let fname = path.file_name().and_then(|s| s.to_str()).unwrap_or("");
            if !fname.ends_with(".timer") {
                continue;
            }
            if !fname.starts_with(&prefix) {
                continue;
            }

            let base = fname.trim_end_matches(".timer");
            let task_name = base.strip_prefix(&prefix).unwrap_or("");

            let _ = self.stop_unit(fname);
            let _ = self.disable_unit(fname);
            let _ = fs::remove_file(&path);

            let one_shot = format!("{base}.service");
            let _ = self.stop_unit(&one_shot);
            let _ = fs::remove_file(unit_dir.join(&one_shot));

            if !task_name.is_empty() {
                if let Ok(tep) = task_env_file_path(service_name, task_name, self.mode) {
                    let _ = fs::remove_file(tep);
                }
                if let Some(tlp) = task_log_file_path(service_name, task_name, self.mode) {
                    let _ = fs::remove_file(tlp);
                }
            }
        }

        Ok(())
    }

    pub fn daemon_reload(&self) -> Result<()> {
        self.exec.run(self.mode, &["daemon-reload"])
    }

    pub(super) fn start_unit(&self, unit: &str) -> Result<()> {
        self.exec.run(self.mode, &["start", unit])
    }

    pub(super) fn stop_unit(&self, unit: &str) -> Result<()> {
        self.exec.run(self.mode, &["stop", unit])
    }

    pub(super) fn enable_unit(&self, unit: &str) -> Result<()> {
        self.exec.run(self.mode, &["enable", unit])
    }

    pub(super) fn disable_unit(&self, unit: &str) -> Result<()> {
        self.exec.run(self.mode, &["disable", unit])
    }
}
