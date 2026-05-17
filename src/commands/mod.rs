//! CLI surface: subcommands map 1:1 to user operations (`clap`).

mod run;

use std::collections::HashMap;
use std::fs;
use std::process::{Command, Stdio};
use std::time::Duration;

use clap::{Parser, Subcommand};
use serde::Serialize;

use crate::error::{Error, Result};
use crate::help_text;
use crate::output;
use crate::sudo;
use crate::systemd::unit_parse::parse_service_file;
use crate::systemd::{
    detect_mode, discover_service_mode, get_log_dir, log_file_path, service_path,
    systemd_unit_name, Manager, SystemdMode,
};

/// Root CLI (matches legacy `smdctl` argv layout: first token is the subcommand).
#[derive(Debug, Parser)]
#[command(
    name = "smdctl",
    about = "Docker-like systemd service CLI (Linux)",
    version = env!("CARGO_PKG_VERSION"),
    disable_help_subcommand = true
)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Option<Commands>,
}

#[derive(Debug, Subcommand)]
#[allow(clippy::large_enum_variant)]
pub enum Commands {
    Run(run::RunArgs),
    Ps(PsArgs),
    Tasks(TasksArgs),
    Start {
        #[arg(required = true)]
        names: Vec<String>,
    },
    Stop {
        #[arg(required = true)]
        names: Vec<String>,
    },
    Restart {
        #[arg(required = true)]
        names: Vec<String>,
    },
    Logs(LogsArgs),
    Status {
        #[arg(required = true)]
        name: String,
    },
    Rm(RmArgs),
    Env {
        #[arg(required = true)]
        name: String,
    },
    Inspect {
        #[arg(required = true)]
        name: String,
    },
    Explain {
        #[arg(trailing_var_arg = true, allow_hyphen_values = true)]
        rest: Vec<String>,
    },
    Migrate,
    Regenerate {
        #[arg(required = true)]
        name: String,
    },
    /// Show structured help (same as running `smdctl` with no args).
    Help {
        #[arg(trailing_var_arg = true)]
        rest: Vec<String>,
    },
    /// Print version.
    Version,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, clap::ValueEnum)]
pub enum OutputFormat {
    Table,
    Json,
}

#[derive(Debug, Parser)]
pub struct PsArgs {
    #[arg(short = 'a', long = "all")]
    pub all: bool,
    #[arg(short = 'q', long = "quiet")]
    pub quiet: bool,
    #[arg(short = 'o', long = "output", value_enum, default_value_t = OutputFormat::Table)]
    pub output: OutputFormat,
}

#[derive(Debug, Parser)]
pub struct TasksArgs {
    #[arg(short = 'a', long = "all")]
    pub all: bool,
    /// Optional service name filter
    pub filter: Option<String>,
    #[arg(short = 'o', long = "output", value_enum, default_value_t = OutputFormat::Table)]
    pub output: OutputFormat,
}

#[derive(Debug, Parser)]
pub struct LogsArgs {
    #[arg(short, long)]
    pub follow: bool,
    #[arg(short = 'n', long = "lines", default_value_t = 50)]
    pub lines: i32,
    #[arg(long)]
    pub since: Option<String>,
    #[arg(long)]
    pub until: Option<String>,
    #[arg(required = true)]
    pub service: String,
}

#[derive(Debug, Parser)]
pub struct RmArgs {
    #[arg(short, long)]
    pub force: bool,
    #[arg(required = true)]
    pub names: Vec<String>,
}

pub fn dispatch() -> Result<()> {
    match Cli::parse().command {
        None => {
            print!("{}", help_text::MAIN);
            Ok(())
        }
        Some(Commands::Help { rest }) => {
            if let Some(cmd) = rest.first() {
                if cmd == "run" {
                    print!("{}", help_text::RUN);
                } else {
                    eprintln!("No detailed help available for command: {cmd}");
                    eprintln!("Run 'smdctl help' for general usage.");
                }
            } else {
                print!("{}", help_text::MAIN);
            }
            Ok(())
        }
        Some(Commands::Version) => {
            println!("smdctl version {}", env!("CARGO_PKG_VERSION"));
            Ok(())
        }
        Some(Commands::Run(a)) => run::execute(a),
        Some(Commands::Ps(a)) => cmd_ps(a),
        Some(Commands::Tasks(a)) => cmd_tasks(a),
        Some(Commands::Start { names }) => cmd_start(&names),
        Some(Commands::Stop { names }) => cmd_stop(&names),
        Some(Commands::Restart { names }) => cmd_restart(&names),
        Some(Commands::Logs(a)) => cmd_logs(a),
        Some(Commands::Status { name }) => cmd_status(&name),
        Some(Commands::Rm(a)) => cmd_rm(a),
        Some(Commands::Env { name }) => cmd_env(&name),
        Some(Commands::Inspect { name }) => cmd_inspect(&name),
        Some(Commands::Explain { rest }) => cmd_explain(&rest),
        Some(Commands::Migrate) => cmd_migrate(),
        Some(Commands::Regenerate { name }) => cmd_regenerate(&name),
    }
}

fn cmd_ps(args: PsArgs) -> Result<()> {
    let mut all_services = Vec::new();

    let user_mgr = Manager::with_mode(SystemdMode::User);
    if let Ok(mut u) = user_mgr.list_services(args.all) {
        for s in &mut u {
            s.mode = SystemdMode::User;
        }
        all_services.extend(u);
    }

    let sys_mgr = Manager::with_mode(SystemdMode::System);
    if let Ok(mut u) = sys_mgr.list_services(args.all) {
        for s in &mut u {
            s.mode = SystemdMode::System;
        }
        all_services.extend(u);
    }

    if args.quiet {
        output::format_services_quiet(&all_services);
    } else {
        match args.output {
            OutputFormat::Table => output::format_services_table(&all_services),
            OutputFormat::Json => output::print_services_json(&all_services)?,
        }
    }
    Ok(())
}

fn cmd_tasks(args: TasksArgs) -> Result<()> {
    let filter = args.filter.as_deref().unwrap_or("");
    let mut tasks = Vec::new();

    let user = Manager::with_mode(SystemdMode::User);
    if let Ok(t) = user.list_tasks(args.all, filter) {
        tasks.extend(t);
    }
    let system = Manager::with_mode(SystemdMode::System);
    if let Ok(t) = system.list_tasks(args.all, filter) {
        tasks.extend(t);
    }

    match args.output {
        OutputFormat::Table => output::format_tasks_table(&tasks),
        OutputFormat::Json => output::print_tasks_json(&tasks)?,
    }
    Ok(())
}

fn cmd_start(names: &[String]) -> Result<()> {
    if names.is_empty() {
        return Err(Error::msg("usage: smdctl start SERVICE [SERVICE...]"));
    }
    let first_mode = discover_service_mode(&names[0])?;
    if matches!(first_mode, SystemdMode::System) && sudo::needs_sudo_for_system() {
        return sudo::reexec_with_sudo();
    }
    for name in names {
        let mode = discover_service_mode(name)?;
        let mgr = Manager::with_mode(mode);
        println!("Starting service {name}...");
        if let Err(e) = mgr.start(name) {
            eprintln!("Error: Failed to start {name}: {e}");
            eprintln!("\nTroubleshooting:");
            eprintln!("  - Check status: smdctl status {name}");
            eprintln!("  - View logs: smdctl logs -n 50 {name}");
            return Err(Error::msg(format!("failed to start {name}")));
        }
        println!("Service {name} started successfully.");
    }
    if names.len() == 1 {
        println!("\nNext steps:");
        println!("  Check status: smdctl status {}", names[0]);
        println!("  View logs:    smdctl logs -f {}", names[0]);
    }
    Ok(())
}

fn cmd_stop(names: &[String]) -> Result<()> {
    if names.is_empty() {
        return Err(Error::msg("usage: smdctl stop SERVICE [SERVICE...]"));
    }
    let first_mode = discover_service_mode(&names[0])?;
    if matches!(first_mode, SystemdMode::System) && sudo::needs_sudo_for_system() {
        return sudo::reexec_with_sudo();
    }
    for name in names {
        let mode = discover_service_mode(name)?;
        let mgr = Manager::with_mode(mode);
        println!("Stopping service {name}...");
        mgr.stop(name)
            .map_err(|e| Error::msg(format!("Failed to stop {name}: {e}")))?;
        println!("Service {name} stopped successfully.");
    }
    Ok(())
}

fn cmd_restart(names: &[String]) -> Result<()> {
    if names.is_empty() {
        return Err(Error::msg("usage: smdctl restart SERVICE [SERVICE...]"));
    }
    let first_mode = discover_service_mode(&names[0])?;
    if matches!(first_mode, SystemdMode::System) && sudo::needs_sudo_for_system() {
        return sudo::reexec_with_sudo();
    }
    for name in names {
        let mode = discover_service_mode(name)?;
        let mgr = Manager::with_mode(mode);
        println!("Restarting service {name}...");
        if let Err(e) = mgr.restart(name) {
            eprintln!("Error: Failed to restart {name}: {e}");
            eprintln!("\nTroubleshooting:");
            eprintln!("  - Check status: smdctl status {name}");
            eprintln!("  - View logs: smdctl logs -n 50 {name}");
            return Err(Error::msg(format!("failed to restart {name}")));
        }
        println!("Service {name} restarted successfully.");
    }
    if names.len() == 1 {
        println!("\nNext steps:");
        println!("  Check status: smdctl status {}", names[0]);
        println!("  View logs:    smdctl logs -f {}", names[0]);
    }
    Ok(())
}

fn cmd_logs(args: LogsArgs) -> Result<()> {
    let mode = discover_service_mode(&args.service)?;
    let mgr = Manager::with_mode(mode);
    if !mgr.service_exists(&args.service) {
        return Err(Error::msg(format!("service not found: {}", args.service)));
    }
    if matches!(mode, SystemdMode::User) {
        return logs_from_file(&args.service, args.follow, args.lines);
    }
    logs_from_journal(
        &args.service,
        args.follow,
        args.lines,
        &args.since,
        &args.until,
    )
}

fn logs_from_file(service_name: &str, follow: bool, lines: i32) -> Result<()> {
    let log_file = log_file_path(service_name, SystemdMode::User)
        .ok_or_else(|| Error::msg("user log path is unavailable (is HOME set?)".to_string()))?;
    if !log_file.exists() {
        return Err(Error::msg(format!(
            "log file not found: {}\n\n\
             The service may not have produced any output yet, or it may need to be regenerated.\n\
             Try: smdctl regenerate {service_name}",
            log_file.display()
        )));
    }
    let mut cmd = Command::new("tail");
    if follow {
        cmd.arg("-f").arg(&log_file);
    } else {
        cmd.arg("-n").arg(lines.to_string()).arg(&log_file);
    }
    cmd.stdin(Stdio::inherit());
    cmd.stdout(Stdio::inherit());
    cmd.stderr(Stdio::inherit());
    cmd.status()
        .map_err(|e| Error::msg(format!("failed to read log file: {e}")))?;
    Ok(())
}

fn logs_from_journal(
    service_name: &str,
    follow: bool,
    lines: i32,
    since: &Option<String>,
    until: &Option<String>,
) -> Result<()> {
    let mut cmd = Command::new("journalctl");
    cmd.arg("-u").arg(systemd_unit_name(service_name));
    if follow {
        cmd.arg("-f");
    } else {
        cmd.arg("-n").arg(lines.to_string());
    }
    if let Some(s) = since {
        cmd.arg("--since").arg(s);
    }
    if let Some(u) = until {
        cmd.arg("--until").arg(u);
    }
    cmd.arg("--no-pager");
    cmd.stdin(Stdio::inherit());
    cmd.stdout(Stdio::inherit());
    cmd.stderr(Stdio::inherit());
    cmd.status()
        .map_err(|e| Error::msg(format!("journalctl failed: {e}")))?;
    Ok(())
}

fn cmd_status(name: &str) -> Result<()> {
    let mode = discover_service_mode(name)?;
    let mgr = Manager::with_mode(mode);
    if !mgr.service_exists(name) {
        return Err(Error::msg(format!("service not found: {name}")));
    }
    let info = mgr
        .get_service_info(name)
        .map_err(|e| Error::msg(format!("get service info: {e}")))?;

    println!("Service: {} ({})", info.name, systemd_unit_name(&info.name));
    println!("Mode:    {mode}");
    println!(
        "Status:  {} ({})",
        info.status,
        if info.sub_state.is_empty() {
            "-"
        } else {
            &info.sub_state
        }
    );
    if info.pid > 0 {
        println!("PID:     {}", info.pid);
    }
    if info.uptime > Duration::ZERO {
        println!(
            "Uptime:  {}",
            format_status_uptime(info.uptime.as_secs() as i64)
        );
    }
    if info.memory_bytes > 0 {
        println!("Memory:  {}", output_format_bytes(info.memory_bytes));
    }
    if !info.ports.is_empty() {
        println!("Ports:   {:?}", info.ports);
    }
    println!();
    println!("Service File: {}", service_path(name, mode)?.display());
    println!(
        "Env File:     {}",
        crate::systemd::env_file_path(name, mode)?.display()
    );
    println!("\n--- systemctl status output ---");
    let status_args = if matches!(mode, SystemdMode::User) {
        vec![
            "--user".into(),
            "status".into(),
            systemd_unit_name(name),
            "--no-pager".into(),
            "-l".into(),
            "-n".into(),
            "10".into(),
        ]
    } else {
        vec![
            "status".into(),
            systemd_unit_name(name),
            "--no-pager".into(),
            "-l".into(),
            "-n".into(),
            "10".into(),
        ]
    };
    let _ = Command::new("systemctl")
        .args(&status_args)
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit())
        .status();

    println!("\n--- Next steps ---");
    println!("  View full logs:       smdctl logs -f {name}");
    println!("  Restart service:      smdctl restart {name}");
    println!("  Edit environment:     smdctl env {name}");
    println!("  Inspect config:       smdctl inspect {name}");
    Ok(())
}

fn format_status_uptime(seconds: i64) -> String {
    if seconds == 0 {
        return "-".into();
    }
    let minutes = seconds / 60;
    let hours = minutes / 60;
    let days = hours / 24;
    if days > 0 {
        format!("{} days {} hours", days, hours % 24)
    } else if hours > 0 {
        format!("{} hours {} minutes", hours, minutes % 60)
    } else if minutes > 0 {
        format!("{} minutes", minutes)
    } else {
        format!("{seconds} seconds")
    }
}

fn output_format_bytes(bytes: u64) -> String {
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

fn cmd_rm(args: RmArgs) -> Result<()> {
    if args.names.is_empty() {
        return Err(Error::msg(
            "usage: smdctl rm [OPTIONS] SERVICE [SERVICE...]",
        ));
    }
    let first_mode = discover_service_mode(&args.names[0])?;
    if matches!(first_mode, SystemdMode::System) && sudo::needs_sudo_for_system() {
        return sudo::reexec_with_sudo();
    }
    for name in &args.names {
        let Ok(mode) = discover_service_mode(name) else {
            println!("Service not found: {name}");
            continue;
        };
        let mgr = Manager::with_mode(mode);
        if !mgr.service_exists(name) {
            println!("Service not found: {name}");
            continue;
        }
        let active = mgr.is_active(name);
        let status_msg = if active { "running" } else { "stopped" };
        if !args.force {
            let msg = format!(
                "Remove service '{name}' ({status_msg})? This will stop and delete the service."
            );
            if !output::prompt_yes_no(&msg, false) {
                println!("Skipping {name}");
                continue;
            }
        }
        println!("Removing service {name}...");
        if let Err(e) = mgr.remove(name) {
            eprintln!("Error: Failed to remove {name}: {e}");
            return Err(Error::msg(format!("failed to remove {name}")));
        }
        println!("Service {name} removed successfully.");
    }
    Ok(())
}

fn cmd_env(name: &str) -> Result<()> {
    let mode = discover_service_mode(name)?;
    let mgr = Manager::with_mode(mode);
    if !mgr.service_exists(name) {
        return Err(Error::msg(format!("service not found: {name}")));
    }
    if matches!(mode, SystemdMode::System) && sudo::needs_sudo_for_system() {
        return sudo::reexec_with_sudo();
    }
    crate::env_edit::edit(name, mode)
}

#[derive(Serialize)]
struct InspectOut {
    name: String,
    description: String,
    status: String,
    substate: String,
    pid: i32,
    uptime: String,
    environment: HashMap<String, String>,
    files: HashMap<String, String>,
    resources: InspectResources,
    service_file_content: String,
}

#[derive(Serialize)]
struct InspectResources {
    memory_bytes: u64,
    listening_ports: Vec<i32>,
}

fn cmd_inspect(name: &str) -> Result<()> {
    let mode = discover_service_mode(name)?;
    let mgr = Manager::with_mode(mode);
    if !mgr.service_exists(name) {
        return Err(Error::msg(format!("service not found: {name}")));
    }
    let info = mgr
        .get_service_info(name)
        .map_err(|e| Error::msg(format!("get service info: {e}")))?;
    let service_file = mgr
        .get_service_file(name)
        .map_err(|e| Error::msg(format!("get service file: {e}")))?;

    let env_path = crate::systemd::env_file_path(name, mode)?;
    let mut env_vars = HashMap::new();
    if let Ok(content) = fs::read_to_string(&env_path) {
        for line in content.lines() {
            let t = line.trim();
            if t.is_empty() || t.starts_with('#') {
                continue;
            }
            if let Some((k, v)) = line.split_once('=') {
                env_vars.insert(k.to_string(), v.to_string());
            }
        }
    }

    let out = InspectOut {
        name: info.name.clone(),
        description: info.description.clone(),
        status: info.status.clone(),
        substate: info.sub_state.clone(),
        pid: info.pid,
        uptime: format!("{:?}", info.uptime),
        environment: env_vars,
        files: HashMap::from([
            (
                "service_file".into(),
                service_path(name, mode)?.to_string_lossy().into_owned(),
            ),
            ("env_file".into(), env_path.to_string_lossy().into_owned()),
        ]),
        resources: InspectResources {
            memory_bytes: info.memory_bytes,
            listening_ports: info.ports.clone(),
        },
        service_file_content: service_file,
    };
    output::print_yaml(&out)?;
    Ok(())
}

fn cmd_explain(rest: &[String]) -> Result<()> {
    if rest.is_empty() {
        return Err(Error::msg("usage: smdctl explain COMMAND [ARGS...]"));
    }
    if rest[0] != "run" {
        return Err(Error::msg(
            "explain is currently only supported for the 'run' command",
        ));
    }
    explain_run(&rest[1..])
}

fn explain_run(args: &[String]) -> Result<()> {
    use crate::systemd::types::Service;

    if args.is_empty() {
        println!("EXPLANATION: smdctl run\n");
        println!("\nThis command creates and starts a new systemd service.");
        println!("You must provide either:");
        println!("  1. A YAML config file with -f flag");
        println!("  2. Service name and command: smdctl run NAME -- COMMAND [ARGS...]");
        return Ok(());
    }

    let force_system = args.iter().any(|a| a == "--system");

    let mut service_name = "myapp".to_string();
    let mut command_line = "/usr/bin/mycommand".to_string();
    let mut env_vars: Vec<String> = Vec::new();

    let sep = args.iter().position(|a| a == "--");
    if let Some(s) = sep {
        if s + 1 < args.len() {
            command_line = args[s + 1..].join(" ");
        }
        let mut i = 0;
        let mut non_flag: Vec<String> = Vec::new();
        while i < s {
            if args[i] == "-e" || args[i] == "--env" {
                if i + 1 < s {
                    env_vars.push(args[i + 1].clone());
                    i += 2;
                    continue;
                }
            } else if args[i].starts_with('-') {
                i += 1;
                continue;
            } else {
                non_flag.push(args[i].clone());
            }
            i += 1;
        }
        if let Some(n) = non_flag.last() {
            service_name = n.clone();
        }
    }

    let mut svc = Service::empty();
    svc.name = service_name.clone();
    svc.description = format!("smdctl managed service: {service_name}");
    let parts: Vec<&str> = command_line.split_whitespace().collect();
    if !parts.is_empty() {
        svc.command = parts[0].to_string();
        svc.args = parts[1..].iter().map(|s| s.to_string()).collect();
    }
    for raw in &env_vars {
        if let Some((k, v)) = raw.split_once('=') {
            svc.environment.insert(k.to_string(), v.to_string());
        }
    }

    let mode = detect_mode(&svc, force_system);

    println!("EXPLANATION: smdctl run {}\n", args.join(" "));
    println!("This command will perform the following steps:\n");
    println!("1. Select systemd mode (default: userspace)");
    println!("   → {mode} mode\n");
    if matches!(mode, SystemdMode::System) {
        println!("2. Check for sudo privileges");
        println!("   → If not root, re-execute with sudo\n");
    }
    let mut step = if matches!(mode, SystemdMode::System) {
        3
    } else {
        2
    };
    if !env_vars.is_empty() {
        println!("{step}. Create environment file");
        println!(
            "   → {}",
            crate::systemd::env_file_path(&service_name, mode)?.display()
        );
        println!("   → Contents:");
        for e in &env_vars {
            println!("     {e}");
        }
        println!();
        step += 1;
    }

    println!("{step}. Generate systemd service file");
    println!("   → {}", service_path(&service_name, mode)?.display());
    println!();

    let systemctl_prefix = if matches!(mode, SystemdMode::User) {
        "systemctl --user"
    } else {
        "systemctl"
    };
    let journalctl_prefix = if matches!(mode, SystemdMode::User) {
        "journalctl --user"
    } else {
        "journalctl"
    };

    println!("{}. Reload systemd daemon", step + 1);
    println!("   → {systemctl_prefix} daemon-reload\n");

    println!("{}. Enable service (start at boot)", step + 2);
    println!("   → {systemctl_prefix} enable smdctl-{service_name}\n");

    println!("{}. Start service immediately", step + 3);
    println!("   → {systemctl_prefix} start smdctl-{service_name}\n");

    let mut svc2 = svc;
    svc2.mode = mode;
    let rendered = crate::systemd::generation::generate_service_file(&svc2)?;
    println!("Generated service file:");
    println!("{}", "─".repeat(60));
    print!("{rendered}");
    println!("{}", "─".repeat(60));
    println!();
    println!("To execute this command:");
    println!("  smdctl run {}", args.join(" "));
    println!();
    println!("To see the result:");
    println!("  smdctl status {service_name}");
    println!("  {systemctl_prefix} status smdctl-{service_name}");
    println!("  {journalctl_prefix} -u smdctl-{service_name} -n 100");
    Ok(())
}

fn cmd_migrate() -> Result<()> {
    println!("Migration from system to user mode");
    println!("{}", "=".repeat(50));
    println!();
    let system_mgr = Manager::with_mode(SystemdMode::System);
    let services = system_mgr.list_services(true).unwrap_or_default();
    if services.is_empty() {
        println!("No system services found to migrate.");
        return Ok(());
    }
    println!(
        "Found {} service(s) running in system mode:\n",
        services.len()
    );
    for svc in &services {
        println!("  - {}", svc.name);
    }
    println!();
    println!("Migration Process:");
    println!("1. For each service, review if it needs privileged ports (< 1024)");
    println!("2. Services without privileged ports can be migrated to user mode");
    println!("3. Remove the system service: smdctl rm <service-name>");
    println!("4. Re-create in user mode: smdctl run ... (without --system flag)");
    println!();
    println!("Note: User mode services require lingering to persist after logout.");
    println!("Enable lingering: sudo loginctl enable-linger $USER");
    println!();
    println!("For services requiring privileged ports, they must remain in system mode.");
    Ok(())
}

fn cmd_regenerate(name: &str) -> Result<()> {
    let mode = discover_service_mode(name)?;
    let path = service_path(name, mode)?;
    let content =
        fs::read_to_string(&path).map_err(|e| Error::msg(format!("read service file: {e}")))?;
    let svc = parse_service_file(&content, name, mode)?;

    if matches!(mode, SystemdMode::User) {
        if let Ok(log_dir) = get_log_dir(mode) {
            let _ = fs::create_dir_all(log_dir);
        }
    }

    let new_content = crate::systemd::generation::generate_service_file(&svc)?;
    fs::write(&path, new_content).map_err(|e| Error::msg(format!("write service file: {e}")))?;

    let mgr = Manager::with_mode(mode);
    mgr.daemon_reload()
        .map_err(|e| Error::msg(format!("daemon reload: {e}")))?;

    println!("Regenerated service file: {}", path.display());
    if let Some(lp) = log_file_path(name, mode) {
        println!("Log file: {}", lp.display());
    }
    println!("\nRestart the service to apply changes:");
    println!("  smdctl restart {name}");
    Ok(())
}
