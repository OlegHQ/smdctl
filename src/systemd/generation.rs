use std::fmt::Write as _;

use crate::error::Result;
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::{abs_command_path, env_file_path, log_file_path};
use crate::systemd::types::Service;

pub fn generate_service_file(svc: &Service) -> Result<String> {
    let mut sb = String::new();

    sb.push_str("[Unit]\n");
    if !svc.description.is_empty() {
        let _ = writeln!(sb, "Description={}", svc.description);
    } else {
        let _ = writeln!(sb, "Description=smdctl managed service: {}", svc.name);
    }

    if !svc.after.is_empty() {
        let _ = writeln!(sb, "After={}", svc.after.join(" "));
    } else if matches!(svc.mode, SystemdMode::User) {
        sb.push_str("After=default.target\n");
    } else {
        sb.push_str("After=network-online.target\n");
    }

    if !svc.wants.is_empty() {
        let _ = writeln!(sb, "Wants={}", svc.wants.join(" "));
    }

    sb.push('\n');
    sb.push_str("[Service]\n");
    sb.push_str("Type=simple\n");

    let command = abs_command_path(&svc.command, &svc.workdir);
    let exec_start = if svc.args.is_empty() {
        command.clone()
    } else {
        format!("{} {}", command, svc.args.join(" "))
    };
    let _ = writeln!(sb, "ExecStart={exec_start}");

    if !svc.restart.is_empty() {
        let _ = writeln!(sb, "Restart={}", svc.restart);
    }
    sb.push_str("RestartSec=5s\n");

    if !svc.workdir.is_empty() {
        let _ = writeln!(sb, "WorkingDirectory={}", svc.workdir);
    }

    if matches!(svc.mode, SystemdMode::System) && !svc.user.is_empty() {
        let _ = writeln!(sb, "User={}", svc.user);
    }

    let env_path = env_file_path(&svc.name, svc.mode)?;
    let _ = writeln!(sb, "EnvironmentFile=-{}", env_path.to_string_lossy());

    if svc.timeout_start > 0 {
        let _ = writeln!(sb, "TimeoutStartSec={}", svc.timeout_start);
    }
    if svc.timeout_stop > 0 {
        let _ = writeln!(sb, "TimeoutStopSec={}", svc.timeout_stop);
    }
    if !svc.kill_mode.is_empty() {
        let _ = writeln!(sb, "KillMode={}", svc.kill_mode);
    }
    if svc.private_tmp {
        sb.push_str("PrivateTmp=true\n");
    }
    if !svc.protect_system.is_empty() {
        let _ = writeln!(sb, "ProtectSystem={}", svc.protect_system);
    }
    if svc.no_new_privileges {
        sb.push_str("NoNewPrivileges=true\n");
    }
    if svc.limit_nofile > 0 {
        let _ = writeln!(sb, "LimitNOFILE={}", svc.limit_nofile);
    }
    if svc.tasks_max > 0 {
        let _ = writeln!(sb, "TasksMax={}", svc.tasks_max);
    }

    if let Some(log) = log_file_path(&svc.name, svc.mode) {
        let lp = log.to_string_lossy();
        let _ = writeln!(sb, "StandardOutput=append:{lp}");
        let _ = writeln!(sb, "StandardError=append:{lp}");
    }

    sb.push('\n');
    sb.push_str("[Install]\n");
    if matches!(svc.mode, SystemdMode::User) {
        sb.push_str("WantedBy=default.target\n");
    } else {
        sb.push_str("WantedBy=multi-user.target\n");
    }

    Ok(sb)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::systemd::types::Service;
    use std::collections::HashMap;

    #[test]
    fn user_mode_default_after_and_logs() {
        let svc = Service {
            mode: SystemdMode::User,
            name: "web".into(),
            description: "".into(),
            command: "/usr/bin/python3".into(),
            args: vec!["-m".into(), "http.server".into()],
            workdir: "/opt".into(),
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
        std::env::set_var("HOME", "/tmp/smdctl-gen-test");
        let out = generate_service_file(&svc).expect("gen");
        assert!(out.contains("After=default.target"));
        assert!(out.contains("ExecStart="));
        assert!(out.contains("StandardOutput=append:"));
    }

    #[test]
    fn system_mode_has_install_and_user_directive() {
        let svc = Service {
            mode: SystemdMode::System,
            name: "api".into(),
            description: "API service".into(),
            command: "/usr/local/bin/api".into(),
            args: vec![],
            workdir: "/srv/api".into(),
            user: "api".into(),
            environment: HashMap::new(),
            restart: "on-failure".into(),
            timeout_start: 30,
            timeout_stop: 30,
            kill_mode: "mixed".into(),
            after: vec!["network.target".into()],
            wants: vec!["network-online.target".into()],
            private_tmp: true,
            protect_system: "strict".into(),
            no_new_privileges: true,
            limit_nofile: 65535,
            tasks_max: 0,
        };
        let out = generate_service_file(&svc).expect("gen");
        assert!(out.contains("Description=API service"));
        assert!(out.contains("WantedBy=multi-user.target"));
        assert!(out.contains("User=api"));
        assert!(out.contains("PrivateTmp=true"));
        assert!(out.contains("ProtectSystem=strict"));
        assert!(out.contains("NoNewPrivileges=true"));
        assert!(out.contains("LimitNOFILE=65535"));
        assert!(out.contains("After=network.target"));
        assert!(out.contains("Wants=network-online.target"));
        assert!(!out.contains("StandardOutput=append:"));
    }
}
