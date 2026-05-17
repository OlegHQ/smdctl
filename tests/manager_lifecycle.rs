//! Drive `Manager` through `FakeSystemctl` to verify the systemctl argv
//! contracts (no real `systemctl` is invoked).
//!
//! Tests that touch the filesystem under `HOME` acquire `HOME_LOCK` first,
//! since `HOME` is process-global. The lock is poison-tolerant so that one
//! panicking test does not cascade-fail the others.

use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Mutex, MutexGuard};

use smdctl::systemd::{FakeSystemctl, Manager, Service, SystemdMode};

static HOME_LOCK: Mutex<()> = Mutex::new(());

fn lock_home() -> MutexGuard<'static, ()> {
    HOME_LOCK
        .lock()
        .unwrap_or_else(|poison| poison.into_inner())
}

struct HomeGuard {
    prev: Option<String>,
    _lock: MutexGuard<'static, ()>,
}

impl HomeGuard {
    fn set(path: &std::path::Path) -> Self {
        let lock = lock_home();
        let prev = std::env::var("HOME").ok();
        std::env::set_var("HOME", path);
        Self { prev, _lock: lock }
    }
}

impl Drop for HomeGuard {
    fn drop(&mut self) {
        match &self.prev {
            Some(p) => std::env::set_var("HOME", p),
            None => std::env::remove_var("HOME"),
        }
    }
}

fn tmp_home(tag: &str) -> PathBuf {
    let pid = std::process::id();
    let nanos = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("smdctl-it-{tag}-{pid}-{nanos}"));
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

fn make_svc(name: &str) -> Service {
    let mut s = Service::empty();
    s.mode = SystemdMode::User;
    s.name = name.into();
    s.description = format!("test service {name}");
    s.command = "/bin/true".into();
    s.workdir = "/".into();
    s.environment = HashMap::new();
    s.restart = "always".into();
    s.timeout_start = 90;
    s.timeout_stop = 30;
    s.kill_mode = "control-group".into();
    s
}

fn manager_with(fake: FakeSystemctl, mode: SystemdMode) -> Manager {
    Manager::with_executor(mode, Box::new(fake))
}

// -------- pure-executor tests (no filesystem) --------

#[test]
fn daemon_reload_user_mode_passes_correct_args() {
    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    mgr.daemon_reload().expect("ok");
    let calls = fake.calls();
    assert_eq!(calls.len(), 1);
    assert!(matches!(calls[0].0, SystemdMode::User));
    assert_eq!(calls[0].1, vec!["daemon-reload"]);
}

#[test]
fn daemon_reload_system_mode_uses_system_mode() {
    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake.clone(), SystemdMode::System);
    mgr.daemon_reload().expect("ok");
    let (mode, args) = &fake.calls()[0];
    assert!(matches!(mode, SystemdMode::System));
    assert_eq!(args, &vec!["daemon-reload".to_string()]);
}

#[test]
fn is_active_dispatches_is_active_argv() {
    let fake = FakeSystemctl::new();
    fake.push_status(true);
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    assert!(mgr.is_active("foo"));
    let (mode, args) = &fake.calls()[0];
    assert!(matches!(mode, SystemdMode::User));
    assert_eq!(args, &vec!["is-active".to_string(), "smdctl-foo".into()]);
}

#[test]
fn is_active_returns_false_when_executor_says_no() {
    let fake = FakeSystemctl::new();
    fake.push_status(false);
    let mgr = manager_with(fake, SystemdMode::User);
    assert!(!mgr.is_active("foo"));
}

#[test]
fn list_services_empty_output_is_empty_vec() {
    let _g = HomeGuard::set(&tmp_home("list"));
    let fake = FakeSystemctl::new();
    fake.push_output("");
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    let services = mgr.list_services(false).expect("list");
    assert!(services.is_empty());
    let (mode, args) = &fake.calls()[0];
    assert!(matches!(mode, SystemdMode::User));
    assert_eq!(args[0], "list-units");
}

#[test]
fn list_tasks_filters_empty_when_no_timers() {
    let fake = FakeSystemctl::new();
    fake.push_output("[]");
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    let tasks = mgr.list_tasks(false, "").expect("list tasks");
    assert!(tasks.is_empty());
    let (mode, args) = &fake.calls()[0];
    assert!(matches!(mode, SystemdMode::User));
    assert_eq!(args[0], "list-timers");
    assert_eq!(
        args.last().map(|s| s.as_str()),
        Some("smdctl-*-task-*.timer")
    );
}

#[test]
fn list_tasks_with_service_filter_passes_specific_pattern() {
    let fake = FakeSystemctl::new();
    fake.push_output("[]");
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    mgr.list_tasks(true, "myapp").expect("list");
    let (_, args) = &fake.calls()[0];
    assert!(args.contains(&"--all".to_string()));
    assert_eq!(
        args.last().map(|s| s.as_str()),
        Some("smdctl-myapp-task-*.timer")
    );
}

// -------- name-validation (no executor or filesystem reached) --------

#[test]
fn create_rejects_invalid_name() {
    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake, SystemdMode::User);
    let mut svc = make_svc("bad name");
    svc.mode = SystemdMode::User;
    assert!(matches!(
        mgr.create(&svc).unwrap_err(),
        smdctl::error::Error::InvalidServiceName
    ));
}

#[test]
fn create_rejects_empty_name() {
    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake, SystemdMode::User);
    let mut svc = make_svc("placeholder");
    svc.name = String::new();
    assert!(matches!(
        mgr.create(&svc).unwrap_err(),
        smdctl::error::Error::EmptyServiceName
    ));
}

// -------- HOME-touching tests --------

#[test]
fn create_writes_unit_file_and_reloads_daemon() {
    let home = tmp_home("create");
    let _g = HomeGuard::set(&home);

    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    let svc = make_svc("itcreate");

    mgr.create(&svc).expect("create");

    let unit = home
        .join(".config/systemd/user")
        .join("smdctl-itcreate.service");
    assert!(unit.exists(), "expected {unit:?} to exist");

    let calls = fake.calls();
    assert!(
        calls
            .iter()
            .any(|(m, a)| matches!(m, SystemdMode::User) && a == &vec!["daemon-reload".to_string()]),
        "expected a user-mode daemon-reload call; got: {calls:?}"
    );
}

#[test]
fn create_rejects_mode_mismatch() {
    let home = tmp_home("mismatch");
    let _g = HomeGuard::set(&home);

    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake, SystemdMode::User);
    let mut svc = make_svc("itmm");
    svc.mode = SystemdMode::System;
    let err = mgr.create(&svc).unwrap_err();
    assert!(err.to_string().contains("does not match"), "got: {err}");
}

#[test]
fn start_errors_when_service_missing() {
    let home = tmp_home("start-missing");
    let _g = HomeGuard::set(&home);

    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    let unique = format!("nosuchsvc{}", std::process::id());
    let err = mgr.start(&unique).unwrap_err();
    assert!(matches!(err, smdctl::error::Error::ServiceNotFound(_)));
    assert!(
        fake.calls().is_empty(),
        "executor must not be invoked when service is missing"
    );
}

#[test]
fn lifecycle_create_start_stop_emits_expected_argv() {
    let home = tmp_home("lifecycle");
    let _g = HomeGuard::set(&home);

    let fake = FakeSystemctl::new();
    let mgr = manager_with(fake.clone(), SystemdMode::User);
    let svc = make_svc("itlife");

    mgr.create(&svc).expect("create");
    mgr.start(&svc.name).expect("start");
    mgr.stop(&svc.name).expect("stop");

    let calls = fake.calls();
    let action_names: Vec<&str> = calls
        .iter()
        .map(|(_, a)| a.first().map(|s| s.as_str()).unwrap_or(""))
        .collect();
    assert!(action_names.contains(&"daemon-reload"));
    assert!(action_names.contains(&"start"));
    assert!(action_names.contains(&"stop"));
}
