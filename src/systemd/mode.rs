use serde::Serialize;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum SystemdMode {
    User,
    System,
}

impl std::fmt::Display for SystemdMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SystemdMode::User => f.write_str("user"),
            SystemdMode::System => f.write_str("system"),
        }
    }
}
