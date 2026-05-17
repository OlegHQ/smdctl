//! Privilege detection and sudo re-exec — isolated from systemd domain.

use std::fs;
use std::os::unix::process::ExitStatusExt;
use std::process::{self, Command, Stdio};

/// Returns true when the current process should escalate for system-wide systemd paths.
pub fn needs_sudo_for_system() -> bool {
    if unsafe { libc::getuid() } == 0 {
        return false;
    }

    let test_file = "/etc/systemd/system/.smdctl-permission-test";
    match fs::File::create(test_file) {
        Ok(f) => {
            drop(f);
            let _ = fs::remove_file(test_file);
            false
        }
        Err(_) => true,
    }
}

/// Re-run this process under `sudo` with the same argv.
pub fn reexec_with_sudo() -> crate::error::Result<()> {
    if unsafe { libc::getuid() } == 0 {
        return Ok(());
    }

    let mut argv = std::env::args();
    let exe = argv
        .next()
        .ok_or_else(|| crate::error::Error::msg("could not read argv[0] for sudo re-exec"))?;
    let rest: Vec<String> = argv.collect();
    let mut cmd = Command::new("sudo");
    cmd.arg(exe);
    for a in rest {
        cmd.arg(a);
    }
    cmd.stdin(Stdio::inherit());
    cmd.stdout(Stdio::inherit());
    cmd.stderr(Stdio::inherit());

    let status = cmd.status().map_err(crate::error::Error::Io)?;
    let code = status
        .code()
        .unwrap_or_else(|| status.signal().map(|s| 128 + s).unwrap_or(1));
    process::exit(code);
}
