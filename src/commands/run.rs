use std::collections::HashMap;
use std::path::PathBuf;
use std::process::{Command, Stdio};

use clap::Parser;

use crate::config::{self, ServiceConfig};
use crate::error::{Error, Result};
use crate::output;
use crate::sudo;
use crate::systemd::types::Service;
use crate::systemd::{detect_mode, LingerChecker, Manager, SystemdMode, Task, TaskSchedule};
use crate::systemd::{env_file_path, log_file_path, service_path, systemd_unit_name};

#[derive(Parser, Debug)]
pub struct RunArgs {
    #[arg(short, long, alias = "file")]
    pub file: Option<PathBuf>,

    #[arg(long)]
    pub system: bool,

    #[arg(short, long = "env")]
    pub env: Vec<String>,

    #[arg(long)]
    pub restart: Option<String>,
    #[arg(long)]
    pub user: Option<String>,
    #[arg(long)]
    pub workdir: Option<String>,
    #[arg(long)]
    pub description: Option<String>,
    #[arg(long)]
    pub timeout_start: Option<i32>,
    #[arg(long)]
    pub timeout_stop: Option<i32>,
    #[arg(long)]
    pub kill_mode: Option<String>,
    #[arg(long)]
    pub private_tmp: bool,
    #[arg(long)]
    pub protect_system: Option<String>,
    #[arg(long)]
    pub no_new_privileges: bool,
    #[arg(long)]
    pub limit_nofile: Option<i32>,
    #[arg(long = "after")]
    pub after: Vec<String>,
    #[arg(long = "wants")]
    pub wants: Vec<String>,

    #[arg(trailing_var_arg = true, allow_hyphen_values = true, hide = true)]
    pub trailing: Vec<String>,
}

pub fn execute(args: RunArgs) -> Result<()> {
    let mut yaml_cfg: Option<ServiceConfig> = None;

    let mut svc: Service = if let Some(ref path) = args.file {
        let cfg = config::load_from_file(path)?;
        yaml_cfg = Some(cfg.clone());
        config_to_service(&cfg)
    } else if let Ok(auto) = config::find_config_file() {
        println!("Found config file: {}", auto.display());
        let cfg = config::load_from_file(&auto)?;
        yaml_cfg = Some(cfg.clone());
        config_to_service(&cfg)
    } else {
        parse_cli_trailing(&args.trailing)?
    };

    apply_overrides(&mut svc, &args);

    if svc.name.is_empty() {
        return Err(Error::msg("service name is required"));
    }
    if svc.command.is_empty() {
        return Err(Error::msg("command is required"));
    }

    svc.mode = detect_mode(&svc, args.system);
    if matches!(svc.mode, SystemdMode::System) {
        if sudo::needs_sudo_for_system() {
            return sudo::reexec_with_sudo();
        }
        println!("Running in system mode (elevated port or --system flag)");
    } else {
        println!("Running in user mode");
        LingerChecker::new().warn_if_disabled();
    }

    let mgr = Manager::with_mode(svc.mode);
    println!("Creating service: {}", svc.name);

    mgr.create(&svc)
        .map_err(|e| Error::msg(format!("create service: {e}")))?;

    println!("Enabling service...");
    mgr.enable(&svc.name)
        .map_err(|e| Error::msg(format!("enable service: {e}")))?;

    println!("Starting service...");
    if let Err(e) = mgr.start(&svc.name) {
        eprintln!("\nError: Failed to start service\n");
        eprintln!("{e}");
        if output::prompt_yes_no("Service failed to start. View logs?", true) {
            show_recent_logs(&svc.name, 10, svc.mode)?;
        }
        return Err(Error::ServiceFailedToStart);
    }

    println!("\n{}\n", systemd_unit_name(&svc.name));
    println!("Service started successfully in {} mode.\n", svc.mode);

    if let Some(ref yml) = yaml_cfg {
        if !yml.tasks.is_empty() {
            println!("\nCreating {} scheduled task(s)...\n", yml.tasks.len());
            for tc in &yml.tasks {
                let sched = TaskSchedule {
                    on_calendar: tc.schedule.on_calendar.clone(),
                    on_boot_sec: tc.schedule.on_boot_sec.clone(),
                    on_startup_sec: tc.schedule.on_startup_sec.clone(),
                    on_unit_active_sec: tc.schedule.on_unit_active_sec.clone(),
                    on_unit_inactive_sec: tc.schedule.on_unit_inactive_sec.clone(),
                    persistent: tc.schedule.persistent.unwrap_or(true),
                    randomized_delay_sec: tc.schedule.randomized_delay_sec.clone(),
                    accuracy_sec: tc.schedule.accuracy_sec.clone(),
                };
                let task = Task {
                    name: tc.name.clone(),
                    description: tc.description.clone(),
                    command: tc.command.clone(),
                    args: tc.args.clone(),
                    workdir: tc.workdir.clone(),
                    environment: tc.environment.clone(),
                    schedule: sched,
                };
                if let Err(e) = mgr.create_task(&svc, &task) {
                    eprintln!("Warning: failed to create task {}: {e}", tc.name);
                }
            }
        }
    }

    println!(
        "Service file: {}",
        service_path(&svc.name, svc.mode)?.display()
    );
    println!(
        "Env file:     {}\n",
        env_file_path(&svc.name, svc.mode)?.display()
    );
    println!("Next steps:");
    println!("  Check status:  smdctl status {}", svc.name);
    println!("  View logs:     smdctl logs -f {}", svc.name);
    println!("  List services: smdctl ps");

    Ok(())
}

fn config_to_service(cfg: &ServiceConfig) -> Service {
    let mut s = Service::empty();
    s.name = cfg.name.clone();
    s.description = cfg.description.clone();
    s.command = cfg.command.clone();
    s.args = cfg.args.clone();
    s.workdir = cfg.workdir.clone();
    s.user = cfg.user.clone();
    s.environment = cfg.environment.clone();
    s.restart = cfg.restart.clone();
    s.timeout_start = cfg.timeout_start;
    s.timeout_stop = cfg.timeout_stop;
    s.kill_mode = cfg.kill_mode.clone();
    s.after = cfg.after.clone();
    s.wants = cfg.wants.clone();
    s.private_tmp = cfg.private_tmp;
    s.protect_system = cfg.protect_system.clone();
    s.no_new_privileges = cfg.no_new_privileges;
    s.limit_nofile = cfg.limit_nofile;
    s.tasks_max = cfg.tasks_max;
    if cfg.system_mode {
        s.mode = SystemdMode::System;
    }
    s
}

fn parse_cli_trailing(trailing: &[String]) -> Result<Service> {
    let Some(sep) = trailing.iter().position(|a| a == "--") else {
        eprintln!("Error: Missing '--' separator before command\n");
        eprintln!("Usage: smdctl run [OPTIONS] NAME -- COMMAND [ARGS...]\n");
        eprintln!("Example:");
        eprintln!("  smdctl run myapp -- /usr/bin/python3 server.py\n");
        eprintln!("The '--' separator is required to separate smdctl options from the command.");
        std::process::exit(1);
    };
    if sep == 0 {
        eprintln!("Error: Missing '--' separator before command\n");
        std::process::exit(1);
    }
    let name = trailing[sep - 1].clone();
    if sep + 1 >= trailing.len() {
        return Err(Error::msg("command is required after '--'"));
    }
    let command = trailing[sep + 1].clone();
    let args: Vec<String> = trailing.iter().skip(sep + 2).cloned().collect();
    let mut svc = Service::empty();
    svc.name = name;
    svc.command = command;
    svc.args = args;
    svc.environment = HashMap::new();
    Ok(svc)
}

fn apply_overrides(svc: &mut Service, args: &RunArgs) {
    if let Some(r) = &args.restart {
        svc.restart = r.clone();
    }
    if let Some(u) = &args.user {
        svc.user = u.clone();
    }
    if let Some(w) = &args.workdir {
        svc.workdir = w.clone();
    }
    if let Some(d) = &args.description {
        svc.description = d.clone();
    }
    if let Some(t) = args.timeout_start {
        svc.timeout_start = t;
    }
    if let Some(t) = args.timeout_stop {
        svc.timeout_stop = t;
    }
    if let Some(k) = &args.kill_mode {
        svc.kill_mode = k.clone();
    }
    if args.private_tmp {
        svc.private_tmp = true;
    }
    if let Some(p) = &args.protect_system {
        svc.protect_system = p.clone();
    }
    if args.no_new_privileges {
        svc.no_new_privileges = true;
    }
    if let Some(l) = args.limit_nofile {
        svc.limit_nofile = l;
    }
    if !args.after.is_empty() {
        svc.after = args.after.clone();
    }
    if !args.wants.is_empty() {
        svc.wants = args.wants.clone();
    }

    if !args.env.is_empty() {
        for raw in &args.env {
            if let Some((k, v)) = raw.split_once('=') {
                svc.environment.insert(k.to_string(), v.to_string());
            }
        }
    }
}

fn show_recent_logs(service_name: &str, lines: i32, mode: SystemdMode) -> Result<()> {
    println!("\nRecent logs (last {lines} lines):");
    println!("{}", "─".repeat(60));
    if let Some(log_file) = log_file_path(service_name, mode) {
        let _ = Command::new("tail")
            .args([
                "-n",
                &lines.to_string(),
                log_file.to_str().unwrap_or_default(),
            ])
            .stdout(Stdio::inherit())
            .stderr(Stdio::inherit())
            .status();
    } else {
        let _ = Command::new("journalctl")
            .args([
                "-u",
                &systemd_unit_name(service_name),
                "-n",
                &lines.to_string(),
                "--no-pager",
            ])
            .stdout(Stdio::inherit())
            .stderr(Stdio::inherit())
            .status();
    }
    println!("{}", "─".repeat(60));
    println!("\nFor full logs, run: smdctl logs {service_name}");
    Ok(())
}
