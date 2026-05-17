use regex::Regex;

use crate::systemd::mode::SystemdMode;
use crate::systemd::types::Service;

pub struct PortDetector {
    env_var_names: Vec<&'static str>,
    port_pattern: Regex,
}

impl PortDetector {
    pub fn new() -> Self {
        Self {
            env_var_names: vec![
                "PORT",
                "BIND_PORT",
                "HTTP_PORT",
                "HTTPS_PORT",
                "SERVER_PORT",
                "LISTEN_PORT",
                "APP_PORT",
            ],
            port_pattern: Regex::new(r"(?:--?port[=\s]|:)(\d+)").expect("port regex"),
        }
    }

    pub fn needs_elevated_port(&self, svc: &Service) -> bool {
        for key in &self.env_var_names {
            if let Some(port_str) = svc.environment.get(*key) {
                if let Ok(port) = port_str.parse::<i32>() {
                    if port > 0 && port < 1024 {
                        return true;
                    }
                }
            }
        }

        let mut full_cmd = svc.command.clone();
        full_cmd.push(' ');
        full_cmd.push_str(&svc.args.join(" "));
        for m in self.port_pattern.captures_iter(&full_cmd) {
            if let Some(n) = m.get(1).map(|g| g.as_str()) {
                if let Ok(port) = n.parse::<i32>() {
                    if port > 0 && port < 1024 {
                        return true;
                    }
                }
            }
        }
        false
    }
}

impl Default for PortDetector {
    fn default() -> Self {
        Self::new()
    }
}

pub fn detect_mode(svc: &Service, force_system: bool) -> SystemdMode {
    if force_system {
        return SystemdMode::System;
    }
    let d = PortDetector::new();
    if d.needs_elevated_port(svc) {
        SystemdMode::System
    } else {
        SystemdMode::User
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn svc(env: &[(&str, &str)], command: &str, args: &[&str]) -> Service {
        let mut s = Service::empty();
        s.environment = env
            .iter()
            .map(|(k, v)| ((*k).into(), (*v).into()))
            .collect();
        s.command = command.into();
        s.args = args.iter().map(|a| (*a).into()).collect();
        s
    }

    #[test]
    fn env_var_with_privileged_port_triggers_system() {
        let s = svc(&[("PORT", "80")], "/bin/true", &[]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn env_var_with_high_port_stays_user() {
        let s = svc(&[("PORT", "8080")], "/bin/true", &[]);
        assert!(!PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn arg_with_dash_port_equals_form() {
        let s = svc(&[], "/bin/srv", &["--port=80"]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn arg_with_dash_port_space_form() {
        let s = svc(&[], "/bin/srv", &["--port", "443"]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn long_port_with_value_after_equals_form() {
        let s = svc(&[], "/bin/srv", &["--port=22"]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn bind_addr_with_privileged_port() {
        let s = svc(&[], "/bin/srv", &["--bind", "0.0.0.0:80"]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn no_port_means_user_mode() {
        let s = svc(&[], "/bin/srv", &["--config", "/etc/x"]);
        assert!(!PortDetector::new().needs_elevated_port(&s));
        assert!(matches!(detect_mode(&s, false), SystemdMode::User));
    }

    #[test]
    fn force_system_overrides_low_port_absence() {
        let s = svc(&[], "/bin/srv", &[]);
        assert!(matches!(detect_mode(&s, true), SystemdMode::System));
    }

    #[test]
    fn alt_env_keys_detected() {
        let s = svc(&[("HTTPS_PORT", "443")], "/bin/srv", &[]);
        assert!(PortDetector::new().needs_elevated_port(&s));
        let s = svc(&[("LISTEN_PORT", "80")], "/bin/srv", &[]);
        assert!(PortDetector::new().needs_elevated_port(&s));
    }

    #[test]
    fn zero_port_does_not_trigger() {
        let s = svc(&[("PORT", "0")], "/bin/srv", &[]);
        assert!(!PortDetector::new().needs_elevated_port(&s));
    }
}
