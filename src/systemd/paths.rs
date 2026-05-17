use std::path::{Path, PathBuf};

use crate::error::{Error, Result};
use crate::systemd::constants::SERVICE_PREFIX;
use crate::systemd::mode::SystemdMode;

fn home_dir() -> Result<PathBuf> {
    std::env::var("HOME")
        .map(PathBuf::from)
        .map_err(|_| Error::msg("HOME environment variable is not set"))
}

pub fn service_file_name(name: &str) -> String {
    format!("{SERVICE_PREFIX}{name}.service")
}

pub fn service_path(name: &str, mode: SystemdMode) -> Result<PathBuf> {
    match mode {
        SystemdMode::System => {
            Ok(PathBuf::from("/etc/systemd/system").join(service_file_name(name)))
        }
        SystemdMode::User => Ok(home_dir()?
            .join(".config/systemd/user")
            .join(service_file_name(name))),
    }
}

pub fn env_file_path(name: &str, mode: SystemdMode) -> Result<PathBuf> {
    match mode {
        SystemdMode::System => Ok(PathBuf::from(format!("/etc/smdctl/env/{name}.env"))),
        SystemdMode::User => Ok(home_dir()?
            .join(".config/smdctl/env")
            .join(format!("{name}.env"))),
    }
}

pub fn get_config_dir(mode: SystemdMode) -> Result<PathBuf> {
    match mode {
        SystemdMode::System => Ok(PathBuf::from("/etc/smdctl/env")),
        SystemdMode::User => Ok(home_dir()?.join(".config/smdctl/env")),
    }
}

pub fn get_log_dir(mode: SystemdMode) -> Result<PathBuf> {
    match mode {
        SystemdMode::System => Ok(PathBuf::from("/var/log/smdctl")),
        SystemdMode::User => Ok(home_dir()?.join(".config/smdctl/logs")),
    }
}

/// Per-service log file (file-based logging, user mode only).
/// Returns `None` in system mode (logs go to journald) or when HOME is unset.
pub fn log_file_path(name: &str, mode: SystemdMode) -> Option<PathBuf> {
    match mode {
        SystemdMode::System => None,
        SystemdMode::User => std::env::var("HOME").ok().map(|h| {
            PathBuf::from(h)
                .join(".config/smdctl/logs")
                .join(format!("{name}.log"))
        }),
    }
}

pub fn systemd_unit_name(name: &str) -> String {
    format!("{SERVICE_PREFIX}{name}")
}

pub fn strip_prefix(full: &str) -> String {
    full.trim_start_matches(SERVICE_PREFIX).to_string()
}

pub fn abs_command_path(command: &str, workdir: &str) -> String {
    let p = Path::new(command);
    if p.is_absolute() {
        command.to_string()
    } else if !workdir.is_empty() && workdir != "/" {
        PathBuf::from(workdir)
            .join(command)
            .to_string_lossy()
            .into_owned()
    } else {
        command.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// HOME is process-global; serialize tests that mutate it so they don't
    /// race. Independent tests that only read HOME don't need the lock.
    static ENV_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());

    fn with_home<F: FnOnce()>(home: Option<&str>, f: F) {
        let _g = ENV_LOCK.lock().unwrap();
        let prev = std::env::var("HOME").ok();
        match home {
            Some(h) => std::env::set_var("HOME", h),
            None => std::env::remove_var("HOME"),
        }
        f();
        match prev {
            Some(p) => std::env::set_var("HOME", p),
            None => std::env::remove_var("HOME"),
        }
    }

    #[test]
    fn service_path_user_mode_uses_home() {
        with_home(Some("/home/test"), || {
            let p = service_path("foo", SystemdMode::User).unwrap();
            assert_eq!(
                p,
                PathBuf::from("/home/test/.config/systemd/user/smdctl-foo.service")
            );
        });
    }

    #[test]
    fn service_path_system_mode_is_absolute() {
        let p = service_path("bar", SystemdMode::System).unwrap();
        assert_eq!(p, PathBuf::from("/etc/systemd/system/smdctl-bar.service"));
    }

    #[test]
    fn service_path_errors_when_home_unset_in_user_mode() {
        with_home(None, || {
            let err = service_path("foo", SystemdMode::User).unwrap_err();
            assert!(err.to_string().contains("HOME"), "got: {err}");
        });
    }

    #[test]
    fn env_file_path_user_mode_uses_home() {
        with_home(Some("/u/me"), || {
            let p = env_file_path("svc", SystemdMode::User).unwrap();
            assert_eq!(p, PathBuf::from("/u/me/.config/smdctl/env/svc.env"));
        });
    }

    #[test]
    fn env_file_path_system_mode_is_absolute() {
        let p = env_file_path("svc", SystemdMode::System).unwrap();
        assert_eq!(p, PathBuf::from("/etc/smdctl/env/svc.env"));
    }

    #[test]
    fn log_file_path_user_mode() {
        with_home(Some("/home/u"), || {
            let p = log_file_path("svc", SystemdMode::User).unwrap();
            assert_eq!(p, PathBuf::from("/home/u/.config/smdctl/logs/svc.log"));
        });
    }

    #[test]
    fn log_file_path_system_mode_is_none() {
        assert!(log_file_path("svc", SystemdMode::System).is_none());
    }

    #[test]
    fn log_file_path_user_mode_without_home_is_none() {
        with_home(None, || {
            assert!(log_file_path("svc", SystemdMode::User).is_none());
        });
    }

    #[test]
    fn systemd_unit_name_adds_prefix() {
        assert_eq!(systemd_unit_name("foo"), "smdctl-foo");
    }

    #[test]
    fn strip_prefix_handles_prefix_and_passthrough() {
        assert_eq!(strip_prefix("smdctl-foo"), "foo");
        assert_eq!(strip_prefix("foo"), "foo");
    }

    #[test]
    fn abs_command_path_passes_absolute() {
        assert_eq!(
            abs_command_path("/usr/bin/echo", "/anywhere"),
            "/usr/bin/echo"
        );
    }

    #[test]
    fn abs_command_path_joins_workdir_for_relative() {
        assert_eq!(abs_command_path("./bin/app", "/srv"), "/srv/./bin/app");
    }

    #[test]
    fn abs_command_path_passes_relative_when_workdir_empty() {
        assert_eq!(abs_command_path("./bin/app", ""), "./bin/app");
        assert_eq!(abs_command_path("./bin/app", "/"), "./bin/app");
    }
}
