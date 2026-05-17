//! Exercise `clap` parsing for every subcommand without executing handlers.

use clap::Parser;
use smdctl::commands::{Cli, Commands};

fn parse(args: &[&str]) -> Cli {
    Cli::try_parse_from(std::iter::once(&"smdctl").chain(args.iter())).expect("parse")
}

#[test]
fn ps_with_all_and_quiet() {
    let cli = parse(&["ps", "-a", "-q"]);
    match cli.command {
        Some(Commands::Ps(a)) => {
            assert!(a.all);
            assert!(a.quiet);
        }
        _ => panic!("expected Ps"),
    }
}

#[test]
fn start_requires_at_least_one_service() {
    let result = Cli::try_parse_from(["smdctl", "start"]);
    assert!(result.is_err());
}

#[test]
fn start_accepts_multiple_names() {
    let cli = parse(&["start", "a", "b", "c"]);
    match cli.command {
        Some(Commands::Start { names }) => {
            assert_eq!(names, vec!["a", "b", "c"]);
        }
        _ => panic!("expected Start"),
    }
}

#[test]
fn stop_and_restart_same_shape() {
    matches!(parse(&["stop", "x"]).command, Some(Commands::Stop { .. }));
    matches!(
        parse(&["restart", "x"]).command,
        Some(Commands::Restart { .. })
    );
}

#[test]
fn logs_defaults_to_50_lines_no_follow() {
    let cli = parse(&["logs", "web"]);
    match cli.command {
        Some(Commands::Logs(a)) => {
            assert_eq!(a.service, "web");
            assert_eq!(a.lines, 50);
            assert!(!a.follow);
        }
        _ => panic!("expected Logs"),
    }
}

#[test]
fn logs_with_follow_and_lines() {
    let cli = parse(&["logs", "-f", "-n", "100", "web"]);
    match cli.command {
        Some(Commands::Logs(a)) => {
            assert!(a.follow);
            assert_eq!(a.lines, 100);
        }
        _ => panic!("expected Logs"),
    }
}

#[test]
fn rm_force_flag_and_multi_names() {
    let cli = parse(&["rm", "-f", "a", "b"]);
    match cli.command {
        Some(Commands::Rm(a)) => {
            assert!(a.force);
            assert_eq!(a.names, vec!["a", "b"]);
        }
        _ => panic!("expected Rm"),
    }
}

#[test]
fn tasks_with_optional_filter() {
    let cli = parse(&["tasks", "myapp"]);
    match cli.command {
        Some(Commands::Tasks(a)) => {
            assert_eq!(a.filter.as_deref(), Some("myapp"));
            assert!(!a.all);
        }
        _ => panic!("expected Tasks"),
    }
}

#[test]
fn tasks_all_flag() {
    let cli = parse(&["tasks", "-a"]);
    match cli.command {
        Some(Commands::Tasks(a)) => {
            assert!(a.all);
            assert!(a.filter.is_none());
        }
        _ => panic!("expected Tasks"),
    }
}

#[test]
fn version_subcommand_recognized() {
    matches!(parse(&["version"]).command, Some(Commands::Version));
}

#[test]
fn no_subcommand_is_legal() {
    let cli = parse(&[]);
    assert!(cli.command.is_none());
}

#[test]
fn explain_captures_trailing_argv() {
    let cli = parse(&["explain", "run", "myapp", "--", "/bin/x"]);
    match cli.command {
        Some(Commands::Explain { rest }) => {
            assert_eq!(rest, vec!["run", "myapp", "--", "/bin/x"]);
        }
        _ => panic!("expected Explain"),
    }
}

#[test]
fn run_parses_env_repeated() {
    let cli = parse(&["run", "-e", "A=1", "-e", "B=2", "myapp", "--", "/bin/x"]);
    match cli.command {
        Some(Commands::Run(a)) => {
            assert_eq!(a.env, vec!["A=1", "B=2"]);
            assert!(a.trailing.iter().any(|s| s == "myapp"));
            assert!(a.trailing.iter().any(|s| s == "/bin/x"));
        }
        _ => panic!("expected Run"),
    }
}

#[test]
fn migrate_help_inspect_env_have_correct_shapes() {
    matches!(parse(&["migrate"]).command, Some(Commands::Migrate));
    matches!(
        parse(&["inspect", "web"]).command,
        Some(Commands::Inspect { .. })
    );
    matches!(parse(&["env", "web"]).command, Some(Commands::Env { .. }));
    matches!(
        parse(&["status", "web"]).command,
        Some(Commands::Status { .. })
    );
    matches!(
        parse(&["regenerate", "web"]).command,
        Some(Commands::Regenerate { .. })
    );
}
