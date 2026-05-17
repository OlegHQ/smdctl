use std::collections::HashMap;
use std::time::Duration;

use serde::Serialize;

use crate::systemd::mode::SystemdMode;

/// Fully-resolved service definition used when rendering units and passing to systemd.
#[derive(Debug, Clone)]
pub struct Service {
    pub mode: SystemdMode,
    pub name: String,
    pub description: String,
    pub command: String,
    pub args: Vec<String>,
    pub workdir: String,
    pub user: String,
    pub environment: HashMap<String, String>,
    pub restart: String,
    pub timeout_start: i32,
    pub timeout_stop: i32,
    pub kill_mode: String,
    pub after: Vec<String>,
    pub wants: Vec<String>,
    pub private_tmp: bool,
    pub protect_system: String,
    pub no_new_privileges: bool,
    pub limit_nofile: i32,
    pub tasks_max: i32,
}

impl Service {
    pub fn empty() -> Self {
        Self {
            mode: SystemdMode::User,
            name: String::new(),
            description: String::new(),
            command: String::new(),
            args: Vec::new(),
            workdir: "/".to_string(),
            user: String::new(),
            environment: HashMap::new(),
            restart: "always".to_string(),
            timeout_start: 90,
            timeout_stop: 30,
            kill_mode: "control-group".to_string(),
            after: Vec::new(),
            wants: Vec::new(),
            private_tmp: false,
            protect_system: String::new(),
            no_new_privileges: false,
            limit_nofile: 0,
            tasks_max: 0,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct ServiceInfo {
    pub name: String,
    pub pid: i32,
    pub status: String,
    pub sub_state: String,
    pub uptime: Duration,
    pub description: String,
    pub memory_bytes: u64,
    pub ports: Vec<i32>,
    pub mode: SystemdMode,
}
