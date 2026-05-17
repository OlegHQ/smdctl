use comfy_table::presets::NOTHING;
use comfy_table::{Attribute, Cell, Color, ContentArrangement, Table};
use serde::Serialize;

use crate::error::Result;
use crate::systemd::{ServiceInfo, TaskInfo};

fn new_table() -> Table {
    let mut t = Table::new();
    t.load_preset(NOTHING)
        .set_content_arrangement(ContentArrangement::Dynamic);
    t
}

fn header_cell(s: &str) -> Cell {
    Cell::new(s).add_attribute(Attribute::Bold)
}

fn status_cell(status: &str) -> Cell {
    let cell = Cell::new(status);
    match status {
        "running" | "active" => cell.fg(Color::Green),
        "auto-restart" | "activating" | "reloading" | "deactivating" => cell.fg(Color::Yellow),
        "failed" => cell.fg(Color::Red),
        "dead" | "inactive" | "stopped" => cell.fg(Color::DarkGrey),
        _ => cell,
    }
}

pub fn format_services_table(services: &[ServiceInfo]) {
    if services.is_empty() {
        println!("No services found.");
        return;
    }
    let mut t = new_table();
    t.set_header(
        ["NAME", "MODE", "PID", "STATUS", "MEMORY", "UPTIME", "PORTS", "DESCRIPTION"]
            .map(header_cell),
    );
    for svc in services {
        let pid = if svc.pid > 0 {
            svc.pid.to_string()
        } else {
            "-".into()
        };
        let memory = if svc.memory_bytes > 0 {
            format_bytes(svc.memory_bytes)
        } else {
            "-".into()
        };
        let uptime = if svc.uptime.as_secs() > 0 {
            format_duration(svc.uptime)
        } else {
            "-".into()
        };
        let status = if !svc.sub_state.is_empty() {
            svc.sub_state.as_str()
        } else {
            svc.status.as_str()
        };
        let ports = if svc.ports.is_empty() {
            "-".into()
        } else {
            format_ports(&svc.ports)
        };
        t.add_row(vec![
            Cell::new(&svc.name),
            Cell::new(svc.mode),
            Cell::new(pid),
            status_cell(status),
            Cell::new(memory),
            Cell::new(uptime),
            Cell::new(ports),
            Cell::new(&svc.description),
        ]);
    }
    println!("{t}");
}

pub fn format_services_quiet(services: &[ServiceInfo]) {
    for s in services {
        println!("{}", s.name);
    }
}

pub fn print_services_json(services: &[ServiceInfo]) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(services)?);
    Ok(())
}

pub fn format_tasks_table(tasks: &[TaskInfo]) {
    if tasks.is_empty() {
        println!("No tasks found.");
        return;
    }
    let mut t = new_table();
    t.set_header(["SERVICE", "TASK", "MODE", "NEXT", "LAST", "TIMER"].map(header_cell));
    for task in tasks {
        t.add_row(vec![
            Cell::new(&task.service),
            Cell::new(&task.task),
            Cell::new(task.mode),
            Cell::new(&task.next),
            Cell::new(&task.last),
            Cell::new(&task.timer_unit),
        ]);
    }
    println!("{t}");
}

pub fn print_tasks_json(tasks: &[TaskInfo]) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(tasks)?);
    Ok(())
}

fn format_ports(ports: &[i32]) -> String {
    if ports.is_empty() {
        return "-".into();
    }
    ports
        .iter()
        .map(|p| p.to_string())
        .collect::<Vec<_>>()
        .join(",")
}

fn format_bytes(bytes: u64) -> String {
    const UNIT: u64 = 1024;
    if bytes < UNIT {
        return format!("{bytes} B");
    }
    let mut div = UNIT;
    let mut exp = 0u32;
    let mut n = bytes / UNIT;
    while n >= UNIT {
        div *= UNIT;
        exp += 1;
        n /= UNIT;
    }
    let c = "KMGTPE".as_bytes()[exp as usize] as char;
    format!("{:.1} {c}B", bytes as f64 / div as f64)
}

fn format_duration(d: std::time::Duration) -> String {
    let seconds = d.as_secs() as i64;
    let minutes = seconds / 60;
    let hours = minutes / 60;
    let days = hours / 24;
    if days > 0 {
        format!("{}d {}h", days, hours % 24)
    } else if hours > 0 {
        format!("{}h {}m", hours, minutes % 60)
    } else if minutes > 0 {
        format!("{}m", minutes)
    } else {
        format!("{seconds}s")
    }
}

pub fn prompt_yes_no(message: &str, default_yes: bool) -> bool {
    use std::io::{self, BufRead, Write};
    let mut prompt = message.to_string();
    prompt.push_str(if default_yes { " [Y/n] " } else { " [y/N] " });
    print!("{prompt}");
    let _ = io::stdout().flush();
    let stdin = io::stdin();
    let mut line = String::new();
    if stdin.lock().read_line(&mut line).is_err() {
        return default_yes;
    }
    let answer = line.trim().to_lowercase();
    if answer.is_empty() {
        return default_yes;
    }
    answer == "y" || answer == "yes"
}

pub fn print_yaml<T: Serialize>(v: &T) -> Result<()> {
    println!("{}", serde_yaml::to_string(v)?);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::systemd::SystemdMode;
    use std::time::Duration;

    #[test]
    fn bytes_under_kib_renders_in_bytes() {
        assert_eq!(format_bytes(0), "0 B");
        assert_eq!(format_bytes(512), "512 B");
        assert_eq!(format_bytes(1023), "1023 B");
    }

    #[test]
    fn bytes_kib_mib_gib_boundaries() {
        assert_eq!(format_bytes(1024), "1.0 KB");
        assert_eq!(format_bytes(1024 * 1024), "1.0 MB");
        assert_eq!(format_bytes(1024 * 1024 * 1024), "1.0 GB");
    }

    #[test]
    fn duration_seconds_minutes_hours_days() {
        assert_eq!(format_duration(Duration::from_secs(5)), "5s");
        assert_eq!(format_duration(Duration::from_secs(90)), "1m");
        assert_eq!(format_duration(Duration::from_secs(3700)), "1h 1m");
        assert_eq!(format_duration(Duration::from_secs(90_000)), "1d 1h");
    }

    #[test]
    fn ports_dash_when_empty() {
        assert_eq!(format_ports(&[]), "-");
    }

    #[test]
    fn ports_join_full_list() {
        assert_eq!(format_ports(&[80]), "80");
        assert_eq!(format_ports(&[80, 443]), "80,443");
        assert_eq!(format_ports(&[1, 2, 3, 4, 5, 6, 7, 8]), "1,2,3,4,5,6,7,8");
    }

    fn sample_service() -> ServiceInfo {
        ServiceInfo {
            name: "always".into(),
            pid: 1234,
            status: "active".into(),
            sub_state: "running".into(),
            uptime: Duration::from_secs(7200),
            description: "smdctl managed service: always".into(),
            memory_bytes: 120 * 1024 * 1024,
            ports: vec![8484],
            mode: SystemdMode::User,
        }
    }

    #[test]
    fn services_table_renders_when_empty_and_populated() {
        format_services_table(&[]);
        format_services_table(&[sample_service()]);
    }

    #[test]
    fn services_json_round_trips() {
        let services = vec![sample_service()];
        let json = serde_json::to_string(&services).unwrap();
        assert!(json.contains("\"name\":\"always\""));
        assert!(json.contains("\"mode\":\"user\""));
    }
}
