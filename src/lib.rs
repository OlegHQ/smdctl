#![cfg(target_os = "linux")]

pub mod commands;
pub mod config;
pub mod env_edit;
pub mod error;
pub mod help_text;
pub mod output;
pub mod sudo;
pub mod systemd;

pub type Result<T> = std::result::Result<T, error::Error>;

pub fn run() -> Result<()> {
    commands::dispatch()
}
