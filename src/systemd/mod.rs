pub mod constants;
pub mod discovery;
pub mod executor;
pub mod generation;
pub mod lifecycle;
pub mod linger;
pub mod manager;
pub mod mode;
pub mod paths;
pub mod port_detect;
pub mod query;
pub mod task_ops;
pub mod tasks;
pub mod types;
pub mod unit_parse;

pub use discovery::discover_service_mode;
pub use executor::{FakeSystemctl, RealSystemctl, SystemctlExec};
pub use linger::LingerChecker;
pub use manager::Manager;
pub use mode::SystemdMode;
pub use paths::{
    env_file_path, get_config_dir, get_log_dir, log_file_path, service_path, strip_prefix,
    systemd_unit_name,
};
pub use port_detect::detect_mode;
pub use tasks::{Task, TaskInfo, TaskSchedule};
pub use types::{Service, ServiceInfo};
