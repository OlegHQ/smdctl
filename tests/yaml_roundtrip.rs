//! Parse each shipped example YAML so format drift doesn't go undetected.

use std::path::PathBuf;

use smdctl::config;

fn example(name: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("examples")
        .join(name)
}

#[test]
fn webapp_yaml_parses() {
    let cfg = config::load_from_file(&example("webapp.yml")).expect("parse webapp.yml");
    assert!(!cfg.name.is_empty(), "webapp.yml must declare a name");
    assert!(!cfg.command.is_empty(), "webapp.yml must declare a command");
}

#[test]
fn nodeapp_yaml_parses() {
    let cfg = config::load_from_file(&example("nodeapp.yml")).expect("parse nodeapp.yml");
    assert!(!cfg.name.is_empty());
    assert!(!cfg.command.is_empty());
}

#[test]
fn worker_yaml_parses() {
    let cfg = config::load_from_file(&example("worker.yml")).expect("parse worker.yml");
    assert!(!cfg.name.is_empty());
    assert!(!cfg.command.is_empty());
}

#[test]
fn defaults_filled_when_yaml_omits_them() {
    let cfg = config::parse_yaml(b"name: minimal\ncommand: /bin/true\n").expect("parse");
    assert_eq!(cfg.restart, "always");
    assert_eq!(cfg.timeout_start, 90);
    assert_eq!(cfg.timeout_stop, 30);
    assert_eq!(cfg.workdir, "/");
}
