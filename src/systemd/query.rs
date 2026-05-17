//! Service queries: list / info / status / file read / active check.

use std::collections::HashSet;
use std::process::Command;
use std::sync::OnceLock;
use std::time::{Duration, SystemTime};

use chrono::{DateTime, NaiveDateTime, TimeZone, Utc};
use regex::Regex;

use crate::error::{Error, Result};
use crate::systemd::manager::Manager;
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::{service_path, strip_prefix, systemd_unit_name};
use crate::systemd::types::ServiceInfo;

impl Manager {
    pub fn get_service_file(&self, name: &str) -> Result<String> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let p = service_path(name, self.mode)?;
        Ok(std::fs::read_to_string(p)?)
    }

    pub fn list_services(&self, all: bool) -> Result<Vec<ServiceInfo>> {
        let base = ["list-units", "--type=service", "--no-pager", "--no-legend"];
        let output = if all {
            self.exec.output(
                self.mode,
                &[base[0], base[1], base[2], base[3], "--all", "smdctl-*"],
            )
        } else {
            self.exec
                .output(self.mode, &[base[0], base[1], base[2], base[3], "smdctl-*"])
        };

        let output = match output {
            Ok(o) => o,
            Err(e) => {
                let es = e.to_string();
                if es.contains("No units found") {
                    return Ok(vec![]);
                }
                return Err(Error::msg(format!("list services: {es}")));
            }
        };

        let mut services = Vec::new();
        for line in output.lines() {
            let line = line.trim();
            if line.is_empty() {
                continue;
            }
            let fields: Vec<&str> = line.split_whitespace().collect();
            if fields.len() < 4 {
                continue;
            }
            let full_name = fields[0].trim_end_matches(".service");
            let name = strip_prefix(full_name);
            match self.get_service_info(&name) {
                Ok(info) => services.push(info),
                Err(_) => services.push(ServiceInfo {
                    name: name.clone(),
                    pid: 0,
                    status: fields[2].to_string(),
                    sub_state: fields.get(3).unwrap_or(&"").to_string(),
                    uptime: Duration::ZERO,
                    description: String::new(),
                    memory_bytes: 0,
                    ports: vec![],
                    mode: self.mode,
                }),
            }
        }
        Ok(services)
    }

    pub fn get_service_info(&self, name: &str) -> Result<ServiceInfo> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);

        let props = [
            "MainPID",
            "ActiveState",
            "SubState",
            "Description",
            "ActiveEnterTimestamp",
            "MemoryCurrent",
        ];

        let mut info = ServiceInfo {
            name: name.to_string(),
            pid: 0,
            status: String::new(),
            sub_state: String::new(),
            uptime: Duration::ZERO,
            description: String::new(),
            memory_bytes: 0,
            ports: vec![],
            mode: self.mode,
        };

        for prop in props {
            let Ok(value) = self.get_service_property(&unit, prop) else {
                continue;
            };
            match prop {
                "MainPID" => {
                    if let Ok(pid) = value.parse::<i32>() {
                        info.pid = pid;
                    }
                }
                "ActiveState" => info.status = value,
                "SubState" => info.sub_state = value,
                "Description" => info.description = value,
                "ActiveEnterTimestamp" => {
                    if !value.is_empty() && value != "n/a" {
                        if let Some(t) = parse_systemd_timestamp(&value) {
                            let now = SystemTime::now();
                            if let Ok(elapsed) = now.duration_since(t) {
                                info.uptime = elapsed;
                            }
                        }
                    }
                }
                "MemoryCurrent" => {
                    if !value.is_empty() && value != "[not set]" {
                        if let Ok(mem) = value.parse::<u64>() {
                            info.memory_bytes = mem;
                        }
                    }
                }
                _ => {}
            }
        }

        if matches!(self.mode, SystemdMode::User) && info.status == "active" && info.pid > 0 {
            info.ports = get_listening_ports(info.pid);
        }

        Ok(info)
    }

    fn get_service_property(&self, unit: &str, property: &str) -> Result<String> {
        let out = self
            .exec
            .output(self.mode, &["show", "-p", property, "--value", unit])?;
        Ok(out.trim().to_string())
    }

    pub fn get_status_text(&self, name: &str) -> Result<String> {
        if !self.service_exists(name) {
            return Err(Error::ServiceNotFound(name.to_string()));
        }
        let unit = systemd_unit_name(name);
        // `systemctl status` exits 3 when a unit is not running, so we accept
        // any output and treat exit code as informational.
        self.exec
            .output(self.mode, &["status", &unit, "--no-pager", "-l"])
            .or_else(|e| Ok(e.to_string()))
    }

    pub fn is_active(&self, name: &str) -> bool {
        let unit = systemd_unit_name(name);
        self.exec.status_ok(self.mode, &["is-active", &unit])
    }
}

fn parse_systemd_timestamp(ts: &str) -> Option<SystemTime> {
    // systemctl emits e.g. "Sat 2026-05-16 21:58:55 UTC" — the trailing "UTC" is
    // a literal, not an offset chrono can parse, so strip it and assume UTC.
    let trimmed = ts.trim().trim_end_matches(" UTC");
    if let Ok(naive) = NaiveDateTime::parse_from_str(trimmed, "%a %Y-%m-%d %H:%M:%S") {
        return Some(Utc.from_utc_datetime(&naive).into());
    }
    DateTime::parse_from_rfc3339(ts).ok().map(|d| d.into())
}

fn get_listening_ports(pid: i32) -> Vec<i32> {
    if pid <= 0 {
        return vec![];
    }
    let pid_str = pid.to_string();
    let mut ports = Vec::new();
    let mut seen = HashSet::new();

    if let Ok(tcp) = Command::new("ss").args(["-H", "-lntp"]).output() {
        append_ports_for_pid(
            &mut ports,
            &mut seen,
            &String::from_utf8_lossy(&tcp.stdout),
            &pid_str,
        );
    }
    if let Ok(udp) = Command::new("ss").args(["-H", "-lnup"]).output() {
        append_ports_for_pid(
            &mut ports,
            &mut seen,
            &String::from_utf8_lossy(&udp.stdout),
            &pid_str,
        );
    }
    ports.sort_unstable();
    ports
}

fn append_ports_for_pid(
    ports: &mut Vec<i32>,
    seen: &mut HashSet<i32>,
    output: &str,
    pid_str: &str,
) {
    static PID_RE: OnceLock<Regex> = OnceLock::new();
    let re = PID_RE.get_or_init(|| Regex::new(r"pid=(\d+)").unwrap());
    for line in output.lines() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        let owner = re
            .captures_iter(line)
            .any(|c| c.get(1).map(|m| m.as_str()) == Some(pid_str));
        if !owner {
            continue;
        }
        let fields: Vec<&str> = line.split_whitespace().collect();
        if fields.len() < 4 {
            continue;
        }
        let port = extract_port(fields[3]);
        if port > 0 && seen.insert(port) {
            ports.push(port);
        }
    }
}

fn extract_port(addr: &str) -> i32 {
    if let Some(idx) = addr.rfind("]:") {
        return addr[idx + 2..].parse().unwrap_or(0);
    }
    if let Some(idx) = addr.rfind(':') {
        return addr[idx + 1..].parse().unwrap_or(0);
    }
    0
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_systemd_timestamp_utc_literal() {
        let parsed = parse_systemd_timestamp("Sat 2026-05-16 21:58:55 UTC")
            .expect("systemctl UTC timestamp must parse");
        let expected = Utc
            .with_ymd_and_hms(2026, 5, 16, 21, 58, 55)
            .single()
            .map(SystemTime::from)
            .unwrap();
        assert_eq!(parsed, expected);
    }

    #[test]
    fn parse_systemd_timestamp_rejects_n_a() {
        assert!(parse_systemd_timestamp("n/a").is_none());
    }
}
