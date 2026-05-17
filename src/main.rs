#![cfg(target_os = "linux")]

fn main() {
    if let Err(e) = smdctl::run() {
        eprintln!("Error: {e}");
        std::process::exit(1);
    }
}
