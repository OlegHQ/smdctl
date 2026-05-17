use std::collections::HashMap;
use std::fs;
use std::path::Path;

use regex::Regex;
use serde::{Deserialize, Serialize};

use crate::error::{Error, Result};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceConfig {
    pub name: String,
    #[serde(default)]
    pub description: String,
    pub command: String,
    #[serde(default)]
    pub args: Vec<String>,
    #[serde(default)]
    pub workdir: String,
    #[serde(default)]
    pub user: String,
    #[serde(default)]
    pub environment: HashMap<String, String>,
    #[serde(default)]
    pub restart: String,
    #[serde(default)]
    pub system_mode: bool,
    #[serde(default)]
    pub timeout_start: i32,
    #[serde(default)]
    pub timeout_stop: i32,
    #[serde(default)]
    pub kill_mode: String,
    #[serde(default)]
    pub after: Vec<String>,
    #[serde(default)]
    pub wants: Vec<String>,
    #[serde(default)]
    pub private_tmp: bool,
    #[serde(default)]
    pub protect_system: String,
    #[serde(default)]
    pub no_new_privileges: bool,
    #[serde(default)]
    pub limit_nofile: i32,
    #[serde(default)]
    pub tasks_max: i32,
    #[serde(default)]
    pub tasks: Vec<TaskConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskConfig {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub description: String,
    #[serde(default)]
    pub command: String,
    #[serde(default)]
    pub args: Vec<String>,
    #[serde(default)]
    pub workdir: String,
    #[serde(default)]
    pub environment: HashMap<String, String>,
    #[serde(default)]
    pub schedule: TaskScheduleYaml,
}

#[derive(Debug, Default, Clone, Serialize, Deserialize)]
pub struct TaskScheduleYaml {
    #[serde(default, rename = "on_calendar")]
    pub on_calendar: String,
    #[serde(default, rename = "on_boot_sec")]
    pub on_boot_sec: String,
    #[serde(default, rename = "on_startup_sec")]
    pub on_startup_sec: String,
    #[serde(default, rename = "on_unit_active_sec")]
    pub on_unit_active_sec: String,
    #[serde(default, rename = "on_unit_inactive_sec")]
    pub on_unit_inactive_sec: String,
    #[serde(default)]
    pub persistent: Option<bool>,
    #[serde(default, rename = "randomized_delay_sec")]
    pub randomized_delay_sec: String,
    #[serde(default, rename = "accuracy_sec")]
    pub accuracy_sec: String,
}

pub fn load_from_file(path: &Path) -> Result<ServiceConfig> {
    let data = fs::read(path).map_err(|e| Error::msg(format!("read config file: {e}")))?;
    let mut cfg = parse_yaml(&data)?;
    expand_env_vars(&mut cfg);
    Ok(cfg)
}

pub fn parse_yaml(data: &[u8]) -> Result<ServiceConfig> {
    let mut cfg: ServiceConfig = serde_yaml::from_slice(data)?;
    if cfg.restart.is_empty() {
        cfg.restart = "always".into();
    }
    if cfg.timeout_start == 0 {
        cfg.timeout_start = 90;
    }
    if cfg.timeout_stop == 0 {
        cfg.timeout_stop = 30;
    }
    if cfg.kill_mode.is_empty() {
        cfg.kill_mode = "control-group".into();
    }
    if cfg.workdir.is_empty() {
        cfg.workdir = "/".into();
    }

    let parent_env = cfg.environment.clone();
    for t in &mut cfg.tasks {
        if t.workdir.is_empty() {
            t.workdir = cfg.workdir.clone();
        }
        for (k, v) in &parent_env {
            t.environment.entry(k.clone()).or_insert_with(|| v.clone());
        }
        if t.schedule.persistent.is_none() {
            t.schedule.persistent = Some(true);
        }
    }

    Ok(cfg)
}

pub fn find_config_file() -> Result<std::path::PathBuf> {
    for path in ["smdctl.yml", "smdctl.yaml"] {
        if Path::new(path).exists() {
            return Ok(path.into());
        }
    }
    Err(Error::msg(
        "config file not found (looked for: smdctl.yml, smdctl.yaml)",
    ))
}

static ENV_VAR_PATTERN: std::sync::OnceLock<Regex> = std::sync::OnceLock::new();

fn expand_string(re: &Regex, s: &str, missing: &mut Vec<String>) -> String {
    re.replace_all(s, |caps: &regex::Captures| {
        let var = caps
            .get(1)
            .map(|m| m.as_str())
            .or_else(|| caps.get(2).map(|m| m.as_str()))
            .unwrap_or("");
        match std::env::var(var) {
            Ok(val) => val,
            Err(_) => {
                missing.push(var.to_string());
                String::new()
            }
        }
    })
    .into_owned()
}

fn expand_env_vars(cfg: &mut ServiceConfig) {
    let re = ENV_VAR_PATTERN.get_or_init(|| {
        Regex::new(r"\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)").expect("env pattern")
    });
    let mut missing: Vec<String> = Vec::new();

    cfg.name = expand_string(re, &cfg.name, &mut missing);
    cfg.description = expand_string(re, &cfg.description, &mut missing);
    cfg.command = expand_string(re, &cfg.command, &mut missing);
    cfg.workdir = expand_string(re, &cfg.workdir, &mut missing);
    cfg.user = expand_string(re, &cfg.user, &mut missing);
    cfg.restart = expand_string(re, &cfg.restart, &mut missing);
    cfg.kill_mode = expand_string(re, &cfg.kill_mode, &mut missing);
    cfg.protect_system = expand_string(re, &cfg.protect_system, &mut missing);
    for a in &mut cfg.args {
        *a = expand_string(re, a, &mut missing);
    }
    for v in &mut cfg.after {
        *v = expand_string(re, v, &mut missing);
    }
    for v in &mut cfg.wants {
        *v = expand_string(re, v, &mut missing);
    }
    let env_kv: Vec<(String, String)> = cfg
        .environment
        .iter()
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect();
    cfg.environment.clear();
    for (k, v) in env_kv {
        cfg.environment
            .insert(k, expand_string(re, &v, &mut missing));
    }
    for t in &mut cfg.tasks {
        t.name = expand_string(re, &t.name, &mut missing);
        t.description = expand_string(re, &t.description, &mut missing);
        t.command = expand_string(re, &t.command, &mut missing);
        t.workdir = expand_string(re, &t.workdir, &mut missing);
        for a in &mut t.args {
            *a = expand_string(re, a, &mut missing);
        }
        let te_kv: Vec<(String, String)> = t
            .environment
            .iter()
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect();
        t.environment.clear();
        for (k, v) in te_kv {
            t.environment.insert(k, expand_string(re, &v, &mut missing));
        }
    }

    missing.sort();
    missing.dedup();
    if !missing.is_empty() {
        println!(
            "Warning: environment variables not set (using empty value): {}",
            missing.join(", ")
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_yaml_applies_defaults() {
        let yaml = b"name: web\ncommand: /bin/x\n";
        let cfg = parse_yaml(yaml).unwrap();
        assert_eq!(cfg.restart, "always");
        assert_eq!(cfg.timeout_start, 90);
        assert_eq!(cfg.timeout_stop, 30);
        assert_eq!(cfg.kill_mode, "control-group");
        assert_eq!(cfg.workdir, "/");
    }

    #[test]
    fn parse_yaml_preserves_explicit_values() {
        let yaml = b"name: web\ncommand: /bin/x\nrestart: no\ntimeout_start: 10\n";
        let cfg = parse_yaml(yaml).unwrap();
        assert_eq!(cfg.restart, "no");
        assert_eq!(cfg.timeout_start, 10);
    }

    #[test]
    fn parse_yaml_with_tasks_inherits_parent_env() {
        let yaml = b"name: web\ncommand: /bin/x\nenvironment:\n  A: '1'\n  B: '2'\ntasks:\n  - name: t\n    command: /bin/y\n    schedule:\n      on_calendar: daily\n";
        let cfg = parse_yaml(yaml).unwrap();
        assert_eq!(cfg.tasks.len(), 1);
        let t = &cfg.tasks[0];
        assert_eq!(t.environment.get("A").map(|s| s.as_str()), Some("1"));
        assert_eq!(t.environment.get("B").map(|s| s.as_str()), Some("2"));
        assert_eq!(t.workdir, "/");
        assert_eq!(t.schedule.persistent, Some(true));
    }

    #[test]
    fn parse_yaml_task_env_overrides_parent() {
        let yaml = b"name: web\ncommand: /bin/x\nenvironment:\n  A: parent\ntasks:\n  - name: t\n    command: /bin/y\n    environment:\n      A: child\n    schedule:\n      on_calendar: daily\n";
        let cfg = parse_yaml(yaml).unwrap();
        assert_eq!(cfg.tasks[0].environment.get("A").unwrap(), "child");
    }

    #[test]
    fn expand_env_vars_replaces_dollar_brace() {
        std::env::set_var("SMDCTL_TEST_DESC", "from-env");
        let mut cfg =
            parse_yaml(b"name: web\ncommand: /bin/x\ndescription: ${SMDCTL_TEST_DESC}\n").unwrap();
        expand_env_vars(&mut cfg);
        assert_eq!(cfg.description, "from-env");
    }

    #[test]
    fn expand_env_vars_replaces_dollar_bare() {
        std::env::set_var("SMDCTL_TEST_NAME", "svcname");
        let mut cfg = parse_yaml(b"name: $SMDCTL_TEST_NAME\ncommand: /bin/x\n").unwrap();
        expand_env_vars(&mut cfg);
        assert_eq!(cfg.name, "svcname");
    }

    #[test]
    fn expand_env_vars_replaces_in_environment_values() {
        std::env::set_var("SMDCTL_TEST_DB", "localhost:5432");
        let mut cfg =
            parse_yaml(b"name: web\ncommand: /bin/x\nenvironment:\n  DB: '${SMDCTL_TEST_DB}'\n")
                .unwrap();
        expand_env_vars(&mut cfg);
        assert_eq!(cfg.environment.get("DB").unwrap(), "localhost:5432");
    }

    #[test]
    fn expand_env_vars_missing_yields_empty() {
        let mut cfg =
            parse_yaml(b"name: web\ncommand: /bin/x\ndescription: '${NEVER_SET_XX}'\n").unwrap();
        expand_env_vars(&mut cfg);
        assert_eq!(cfg.description, "");
    }
}
