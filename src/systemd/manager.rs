//! [`Manager`] is the entry point for all systemd operations.
//!
//! It owns the [`SystemctlExec`] subprocess seam and the active [`SystemdMode`].
//! Concrete behavior lives in sibling modules:
//!
//! - [`super::lifecycle`] — create / remove / start / stop / enable / disable.
//! - [`super::query`]     — list / status / info / file read.
//! - [`super::task_ops`]  — scheduled-task create / list / remove.

use std::sync::OnceLock;

use regex::Regex;

use crate::error::{Error, Result};
use crate::systemd::executor::{RealSystemctl, SystemctlExec};
use crate::systemd::mode::SystemdMode;

pub struct Manager {
    pub(super) mode: SystemdMode,
    pub(super) exec: Box<dyn SystemctlExec>,
}

impl Manager {
    pub fn new() -> Self {
        Self::with_mode(SystemdMode::User)
    }

    pub fn with_mode(mode: SystemdMode) -> Self {
        Self {
            mode,
            exec: Box::new(RealSystemctl::new()),
        }
    }

    /// Test seam: build a manager with a custom subprocess executor.
    pub fn with_executor(mode: SystemdMode, exec: Box<dyn SystemctlExec>) -> Self {
        Self { mode, exec }
    }

    pub fn mode(&self) -> SystemdMode {
        self.mode
    }
}

impl Default for Manager {
    fn default() -> Self {
        Self::new()
    }
}

/// Service names accept ASCII alphanumerics, dashes, and underscores only.
/// Reused by `lifecycle::create` and `task_ops::create_task`.
pub(super) fn validate_service_name(name: &str) -> Result<()> {
    if name.is_empty() {
        return Err(Error::EmptyServiceName);
    }
    static NAME_RE: OnceLock<Regex> = OnceLock::new();
    let re = NAME_RE.get_or_init(|| Regex::new(r"^[a-zA-Z0-9_-]+$").unwrap());
    if !re.is_match(name) {
        return Err(Error::InvalidServiceName);
    }
    Ok(())
}
