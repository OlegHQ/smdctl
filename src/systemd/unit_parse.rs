use std::collections::HashMap;

use crate::error::{Error, Result};
use crate::systemd::mode::SystemdMode;
use crate::systemd::types::Service;

pub fn parse_service_file(content: &str, name: &str, mode: SystemdMode) -> Result<Service> {
    let mut svc = Service::empty();
    svc.name = name.to_string();
    svc.mode = mode;
    svc.environment = HashMap::new();

    for line in content.lines() {
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') || line.starts_with('[') {
            continue;
        }
        let Some((key, value)) = line.split_once('=') else {
            continue;
        };
        let key = key.trim();
        let value = value.trim();
        match key {
            "Description" => svc.description = value.to_string(),
            "ExecStart" => {
                let parts: Vec<&str> = value.split_whitespace().collect();
                if !parts.is_empty() {
                    svc.command = parts[0].to_string();
                    if parts.len() > 1 {
                        svc.args = parts[1..].iter().map(|s| s.to_string()).collect();
                    }
                }
            }
            "WorkingDirectory" => svc.workdir = value.to_string(),
            "User" => svc.user = value.to_string(),
            "Restart" => svc.restart = value.to_string(),
            "TimeoutStartSec" => {
                if let Ok(v) = value.parse::<i32>() {
                    svc.timeout_start = v;
                }
            }
            "TimeoutStopSec" => {
                if let Ok(v) = value.parse::<i32>() {
                    svc.timeout_stop = v;
                }
            }
            "KillMode" => svc.kill_mode = value.to_string(),
            "PrivateTmp" => {
                svc.private_tmp = value == "true" || value == "yes";
            }
            "ProtectSystem" => svc.protect_system = value.to_string(),
            "NoNewPrivileges" => {
                svc.no_new_privileges = value == "true" || value == "yes";
            }
            "LimitNOFILE" => {
                if let Ok(v) = value.parse::<i32>() {
                    svc.limit_nofile = v;
                }
            }
            "TasksMax" => {
                if let Ok(v) = value.parse::<i32>() {
                    svc.tasks_max = v;
                }
            }
            "After" => {
                svc.after = value.split_whitespace().map(|s| s.to_string()).collect();
            }
            "Wants" => {
                svc.wants = value.split_whitespace().map(|s| s.to_string()).collect();
            }
            _ => {}
        }
    }

    if svc.command.is_empty() {
        return Err(Error::msg("no ExecStart found in service file"));
    }

    Ok(svc)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::systemd::generation::generate_service_file;
    use crate::systemd::types::Service;

    #[test]
    fn parse_minimal_service_file_succeeds() {
        let content = "[Unit]\nDescription=test\n[Service]\nExecStart=/bin/true\n";
        let svc = parse_service_file(content, "x", SystemdMode::User).unwrap();
        assert_eq!(svc.name, "x");
        assert_eq!(svc.description, "test");
        assert_eq!(svc.command, "/bin/true");
        assert!(matches!(svc.mode, SystemdMode::User));
    }

    #[test]
    fn parse_missing_exec_start_errors() {
        let content = "[Unit]\nDescription=test\n[Service]\nRestart=always\n";
        let err = parse_service_file(content, "x", SystemdMode::User).unwrap_err();
        assert!(err.to_string().contains("ExecStart"), "got: {err}");
    }

    #[test]
    fn parse_exec_start_splits_command_and_args() {
        let content = "[Service]\nExecStart=/bin/echo hello world\n";
        let svc = parse_service_file(content, "x", SystemdMode::User).unwrap();
        assert_eq!(svc.command, "/bin/echo");
        assert_eq!(svc.args, vec!["hello", "world"]);
    }

    #[test]
    fn parse_handles_all_security_directives() {
        let content = "[Service]\nExecStart=/bin/x\nPrivateTmp=true\nProtectSystem=strict\n\
                       NoNewPrivileges=yes\nLimitNOFILE=4096\nKillMode=mixed\nUser=app\n";
        let svc = parse_service_file(content, "x", SystemdMode::System).unwrap();
        assert!(svc.private_tmp);
        assert_eq!(svc.protect_system, "strict");
        assert!(svc.no_new_privileges);
        assert_eq!(svc.limit_nofile, 4096);
        assert_eq!(svc.kill_mode, "mixed");
        assert_eq!(svc.user, "app");
    }

    #[test]
    fn parse_after_wants_lists() {
        let content =
            "[Unit]\nAfter=a.target b.target\nWants=c.target\n[Service]\nExecStart=/bin/x\n";
        let svc = parse_service_file(content, "x", SystemdMode::User).unwrap();
        assert_eq!(svc.after, vec!["a.target", "b.target"]);
        assert_eq!(svc.wants, vec!["c.target"]);
    }

    #[test]
    fn generate_then_parse_round_trips_core_fields() {
        std::env::set_var("HOME", "/tmp/smdctl-roundtrip");
        let mut original = Service::empty();
        original.mode = SystemdMode::User;
        original.name = "round".into();
        original.description = "roundtrip test".into();
        original.command = "/usr/bin/python3".into();
        original.args = vec!["-m".into(), "http.server".into(), "8080".into()];
        original.workdir = "/opt".into();
        original.restart = "on-failure".into();
        original.timeout_start = 45;
        original.timeout_stop = 15;
        original.kill_mode = "mixed".into();
        original.private_tmp = true;
        original.protect_system = "full".into();
        original.no_new_privileges = true;
        original.limit_nofile = 1024;
        original.after = vec!["network.target".into()];

        let unit = generate_service_file(&original).expect("gen");
        let parsed = parse_service_file(&unit, "round", SystemdMode::User).unwrap();

        assert_eq!(parsed.description, original.description);
        assert_eq!(parsed.command, original.command);
        assert_eq!(parsed.args, original.args);
        assert_eq!(parsed.workdir, original.workdir);
        assert_eq!(parsed.restart, original.restart);
        assert_eq!(parsed.timeout_start, original.timeout_start);
        assert_eq!(parsed.timeout_stop, original.timeout_stop);
        assert_eq!(parsed.kill_mode, original.kill_mode);
        assert_eq!(parsed.private_tmp, original.private_tmp);
        assert_eq!(parsed.protect_system, original.protect_system);
        assert_eq!(parsed.no_new_privileges, original.no_new_privileges);
        assert_eq!(parsed.limit_nofile, original.limit_nofile);
        assert_eq!(parsed.after, original.after);
    }

    #[test]
    fn parse_ignores_comments_and_section_headers() {
        let content = "# top comment\n[Unit]\n# inside unit\nDescription=ok\n\
                       [Service]\nExecStart=/bin/x\n# trailing\n";
        let svc = parse_service_file(content, "x", SystemdMode::User).unwrap();
        assert_eq!(svc.description, "ok");
        assert_eq!(svc.command, "/bin/x");
    }
}
