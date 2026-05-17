//! Domain-oriented errors (`thiserror`) — kept small and mapped at the CLI boundary.

use std::io;

use thiserror::Error;

#[derive(Debug, Error)]
pub enum Error {
    #[error("service not found: {0}")]
    ServiceNotFound(String),

    #[error("service already exists: {0}")]
    ServiceAlreadyExists(String),

    #[error(
        "invalid service name: must contain only alphanumeric characters, hyphens, and underscores"
    )]
    InvalidServiceName,

    #[error("service name cannot be empty")]
    EmptyServiceName,

    #[error("service failed to start")]
    ServiceFailedToStart,

    #[error("{0}")]
    Message(String),

    #[error(transparent)]
    Io(#[from] io::Error),

    #[error(transparent)]
    Yaml(#[from] serde_yaml::Error),

    #[error(transparent)]
    Json(#[from] serde_json::Error),
}

impl Error {
    pub fn msg(s: impl Into<String>) -> Self {
        Self::Message(s.into())
    }
}

pub type Result<T> = std::result::Result<T, Error>;
