use std::process::Command;

pub struct LingerChecker;

impl LingerChecker {
    pub fn new() -> Self {
        Self
    }

    pub fn is_enabled(&self) -> crate::error::Result<bool> {
        let username = std::env::var("USER")
            .map_err(|_| crate::error::Error::msg("USER environment variable not set"))?;
        let output = Command::new("loginctl")
            .args(["show-user", &username, "--property=Linger", "--value"])
            .output();
        let Ok(out) = output else {
            return Ok(false);
        };
        Ok(String::from_utf8_lossy(&out.stdout).trim() == "yes")
    }

    pub fn warn_if_disabled(&self) {
        match self.is_enabled() {
            Ok(true) | Err(_) => {}
            Ok(false) => {
                if let Ok(user) = std::env::var("USER") {
                    println!();
                    println!("⚠️  WARNING: User lingering is not enabled for {user}");
                    println!("   Your services will stop when you log out.");
                    println!("   To enable lingering (services persist after logout):");
                    println!("   sudo loginctl enable-linger {user}");
                    println!();
                }
            }
        }
    }
}

impl Default for LingerChecker {
    fn default() -> Self {
        Self::new()
    }
}
