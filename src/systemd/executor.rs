//! Subprocess seam for `systemctl` invocations.
//!
//! [`Manager`](super::Manager) drives systemd through this trait so tests can
//! substitute a [`FakeSystemctl`] without spawning child processes.

use std::process::{Command, Stdio};

use crate::error::{Error, Result};
use crate::systemd::mode::SystemdMode;

pub trait SystemctlExec {
    fn run(&self, mode: SystemdMode, args: &[&str]) -> Result<()>;
    fn output(&self, mode: SystemdMode, args: &[&str]) -> Result<String>;
    fn status_ok(&self, mode: SystemdMode, args: &[&str]) -> bool;
}

pub struct RealSystemctl;

impl RealSystemctl {
    pub fn new() -> Self {
        Self
    }
}

impl Default for RealSystemctl {
    fn default() -> Self {
        Self::new()
    }
}

fn argv<'a>(mode: SystemdMode, args: &'a [&'a str]) -> Vec<&'a str> {
    let mut v = Vec::with_capacity(args.len() + 1);
    if matches!(mode, SystemdMode::User) {
        v.push("--user");
    }
    v.extend_from_slice(args);
    v
}

impl SystemctlExec for RealSystemctl {
    fn run(&self, mode: SystemdMode, args: &[&str]) -> Result<()> {
        let argv = argv(mode, args);
        let output = Command::new("systemctl")
            .args(&argv)
            .stderr(Stdio::piped())
            .output()
            .map_err(Error::Io)?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(Error::msg(format!(
                "systemctl {}: {}\n{stderr}",
                argv.join(" "),
                output.status
            )));
        }
        Ok(())
    }

    fn output(&self, mode: SystemdMode, args: &[&str]) -> Result<String> {
        let argv = argv(mode, args);
        let output = Command::new("systemctl")
            .args(&argv)
            .output()
            .map_err(Error::Io)?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(Error::msg(format!(
                "systemctl {}: {}\n{stderr}",
                argv.join(" "),
                output.status
            )));
        }
        Ok(String::from_utf8_lossy(&output.stdout).into_owned())
    }

    fn status_ok(&self, mode: SystemdMode, args: &[&str]) -> bool {
        let argv = argv(mode, args);
        Command::new("systemctl")
            .args(&argv)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .map(|s| s.success())
            .unwrap_or(false)
    }
}

// -------------------- test fake --------------------

use std::cell::RefCell;
use std::collections::VecDeque;
use std::rc::Rc;

#[derive(Default)]
struct FakeInner {
    calls: RefCell<Vec<(SystemdMode, Vec<String>)>>,
    run_results: RefCell<VecDeque<Result<()>>>,
    output_results: RefCell<VecDeque<Result<String>>>,
    status_results: RefCell<VecDeque<bool>>,
}

/// Recording double for [`SystemctlExec`]. `Clone` gives every handle access to the
/// same recorded calls and queued responses via `Rc<RefCell<…>>`.
#[derive(Clone, Default)]
pub struct FakeSystemctl {
    inner: Rc<FakeInner>,
}

impl FakeSystemctl {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn push_run_ok(&self) {
        self.inner.run_results.borrow_mut().push_back(Ok(()));
    }

    pub fn push_run_err(&self, msg: impl Into<String>) {
        self.inner
            .run_results
            .borrow_mut()
            .push_back(Err(Error::msg(msg.into())));
    }

    pub fn push_output(&self, s: impl Into<String>) {
        self.inner
            .output_results
            .borrow_mut()
            .push_back(Ok(s.into()));
    }

    pub fn push_output_err(&self, msg: impl Into<String>) {
        self.inner
            .output_results
            .borrow_mut()
            .push_back(Err(Error::msg(msg.into())));
    }

    pub fn push_status(&self, ok: bool) {
        self.inner.status_results.borrow_mut().push_back(ok);
    }

    pub fn calls(&self) -> Vec<(SystemdMode, Vec<String>)> {
        self.inner.calls.borrow().clone()
    }

    pub fn call_args(&self) -> Vec<Vec<String>> {
        self.inner
            .calls
            .borrow()
            .iter()
            .map(|(_, a)| a.clone())
            .collect()
    }

    pub fn clear(&self) {
        self.inner.calls.borrow_mut().clear();
        self.inner.run_results.borrow_mut().clear();
        self.inner.output_results.borrow_mut().clear();
        self.inner.status_results.borrow_mut().clear();
    }
}

impl SystemctlExec for FakeSystemctl {
    fn run(&self, mode: SystemdMode, args: &[&str]) -> Result<()> {
        self.inner
            .calls
            .borrow_mut()
            .push((mode, args.iter().map(|s| s.to_string()).collect()));
        self.inner
            .run_results
            .borrow_mut()
            .pop_front()
            .unwrap_or(Ok(()))
    }

    fn output(&self, mode: SystemdMode, args: &[&str]) -> Result<String> {
        self.inner
            .calls
            .borrow_mut()
            .push((mode, args.iter().map(|s| s.to_string()).collect()));
        self.inner
            .output_results
            .borrow_mut()
            .pop_front()
            .unwrap_or_else(|| Ok(String::new()))
    }

    fn status_ok(&self, mode: SystemdMode, args: &[&str]) -> bool {
        self.inner
            .calls
            .borrow_mut()
            .push((mode, args.iter().map(|s| s.to_string()).collect()));
        self.inner
            .status_results
            .borrow_mut()
            .pop_front()
            .unwrap_or(false)
    }
}
