use std::process::{Command, Stdio};

use crate::error::{Error, Result};
use crate::systemd::{env_file_path, get_config_dir, SystemdMode};

pub fn edit(service_name: &str, mode: SystemdMode) -> Result<()> {
    let env_path = env_file_path(service_name, mode)?;

    if !env_path.exists() {
        let env_dir = get_config_dir(mode)?;
        std::fs::create_dir_all(&env_dir)?;

        let content = format!(
            "# Environment variables for service: {service_name}\n# Format: KEY=VALUE (one per line)\n\n"
        );
        std::fs::write(&env_path, content)?;
    }

    let editor = std::env::var("EDITOR").unwrap_or_else(|_| "nano".into());

    let mut cmd = Command::new(&editor);
    cmd.arg(&env_path);
    cmd.stdin(Stdio::inherit());
    cmd.stdout(Stdio::inherit());
    cmd.stderr(Stdio::inherit());

    cmd.status()
        .map_err(|e| Error::msg(format!("editor failed: {e}")))?;

    println!();
    println!("Environment file updated: {}", env_path.display());
    println!();
    println!("To apply changes, restart the service:");
    println!("  smdctl restart {service_name}");

    Ok(())
}
