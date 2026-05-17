use crate::error::{Error, Result};
use crate::systemd::mode::SystemdMode;
use crate::systemd::paths::service_path;

pub fn discover_service_mode(name: &str) -> Result<SystemdMode> {
    if let Ok(p) = service_path(name, SystemdMode::User) {
        if p.exists() {
            return Ok(SystemdMode::User);
        }
    }
    if let Ok(p) = service_path(name, SystemdMode::System) {
        if p.exists() {
            return Ok(SystemdMode::System);
        }
    }
    Err(Error::msg(format!("service not found: {name}")))
}
