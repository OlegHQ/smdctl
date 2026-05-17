# smdctl - Systemd Control CLI

A Docker-like CLI interface for managing systemd services. Designed to be AI-friendly with comprehensive help output that serves as structured guidance for LLM agents.

## Features

- 🐳 **Docker-like syntax** - Familiar commands for systemd management
- 🤖 **AI-friendly** - Help output designed as prompts for LLM agents
- 📝 **YAML configuration** - Define services like docker-compose
- 🔐 **Automatic privilege escalation** - Prompts for sudo only when needed (system mode)
- 📊 **Resource monitoring** - View CPU and memory usage
- 🎯 **Service lifecycle** - Create, start, stop, restart, remove
- 📋 **Environment management** - Easy environment variable editing
- 🔍 **Service inspection** - YAML output of service configuration

## Installation

```bash
# Clone the repository
git clone https://github.com/nexo-tech/smdctl.git
cd smdctl

# Build and install
make install

# Or just build
make build
./target/release/smdctl help
```

## Quick Start

### Create and start a service

```bash
# Run a Python web server
smdctl run webapp -e PORT=8000 -- /usr/bin/python3 -m http.server 8000

# Run a Node.js application
smdctl run nodeapp \
  -e NODE_ENV=production \
  -e PORT=3000 \
  --restart always \
  --workdir /opt/nodeapp \
  -- /usr/bin/node server.js
```

### List services

```bash
# Show running services
smdctl ps

# Show all services
smdctl ps -a
```

### View logs

```bash
# View last 50 lines
smdctl logs webapp

# Follow logs (live tail)
smdctl logs -f webapp

# Show last 100 lines
smdctl logs -n 100 webapp
```

### Control services

```bash
# Start a service
smdctl start webapp

# Stop a service
smdctl stop webapp

# Restart a service
smdctl restart webapp

# Remove a service
smdctl rm webapp
```

## YAML Configuration

Create a `smdctl.yml` file to define your service:

```yaml
name: webapp
description: My web application
command: /usr/bin/python3
args:
  - /opt/app/server.py
  - --port
  - "8080"
workdir: /opt/app
user: webapp
environment:
  PORT: "8080"
  DEBUG: "true"
  DB_HOST: localhost
restart: always
timeout_start: 90
timeout_stop: 30
private_tmp: true
protect_system: full
limit_nofile: 65536

# Optional scheduled tasks (systemd timers)
tasks:
  - name: cleanup
    description: Daily cleanup
    command: /usr/bin/python3
    args:
      - -m
      - app.cleanup
    schedule:
      on_calendar: daily

  - name: report
    command: /opt/app/bin/report
    schedule:
      on_unit_active_sec: 6h
```

Then run:

```bash
# Run from config file
smdctl run -f smdctl.yml

# Override YAML values with CLI flags
smdctl run -f smdctl.yml -e PORT=9090

# Force system mode (required for privileged ports <1024)
smdctl run --system -f smdctl.yml
```

## Commands

### Core Commands

- `run` - Create and start a new service
- `ps` - List services
- `tasks` - List scheduled tasks (systemd timers)
- `start` - Start one or more services
- `stop` - Stop one or more services
- `restart` - Restart one or more services
- `rm` - Remove a service
- `logs` - View service logs
- `status` - Show detailed service status
- `env` - Edit environment variables
- `inspect` - Show service configuration (YAML)
- `explain` - Show what a command will do (dry-run)

### Run Command Options

```
-f, --file FILE              YAML config file
-e, --env KEY=VALUE          Environment variable (repeatable)
--restart POLICY             Restart policy: no|on-failure|always (default: always)
--user USER                  Run as specific user
--workdir PATH               Working directory
--description TEXT           Service description
--timeout-start SECONDS      Startup timeout (default: 90)
--timeout-stop SECONDS       Stop timeout (default: 30)
--kill-mode MODE             Kill mode: control-group|process|mixed
--private-tmp                Use private /tmp directory
--protect-system LEVEL       Protect system: no|strict|full
--no-new-privileges          Prevent privilege escalation
--limit-nofile N             File descriptor limit
--after TARGET               systemd After= dependency (repeatable)
--wants TARGET               systemd Wants= dependency (repeatable)
```

## AI Assistant Workflow Guidance

This tool is designed to be AI-friendly. Here's the typical workflow for AI agents:

### After creating a service

```bash
smdctl status <name>    # Check if started successfully
smdctl logs -f <name>   # View live logs
smdctl ps               # List all services
```

### If a service fails to start

```bash
smdctl status <name>      # Check detailed status
smdctl logs -n 50 <name>  # View recent logs
smdctl env <name>         # Check environment variables
smdctl inspect <name>     # View full configuration
```

### To modify a service

```bash
smdctl stop <name>     # Stop the service
smdctl env <name>      # Edit environment variables
smdctl start <name>    # Start with new settings
```

### Troubleshooting

```bash
# 1. Check service state
smdctl status <name>

# 2. View recent logs
smdctl logs -n 100 <name>

# 3. Inspect full configuration
smdctl inspect <name>

# 4. Raw systemd status (userspace default)
systemctl --user status smdctl-<name>

# 5. Raw journal logs (userspace default)
journalctl --user -u smdctl-<name> -n 100

# If the service is running in system mode:
# systemctl status smdctl-<name>
# journalctl -u smdctl-<name> -n 100
```

## How It Works

- Default: deploys in userspace (systemd `--user`)
- Switches to system mode only for privileged ports (`<1024`) or `--system`
- Userspace unit files: `~/.config/systemd/user/smdctl-<name>.service`
- Userspace env files: `~/.config/smdctl/env/<name>.env`
- System mode unit files: `/etc/systemd/system/smdctl-<name>.service`
- System mode env files: `/etc/smdctl/env/<name>.env`
- All services are prefixed with `smdctl-` to avoid conflicts
- Standard `systemctl`/`journalctl` commands are used under the hood

## Examples

See the `examples/` directory for sample YAML configurations:

- `webapp.yml` - Python Flask application
- `nodeapp.yml` - Node.js API service
- `worker.yml` - Background job processor

## Requirements

- Linux with systemd
- Rust 1.74+ (for building)
- sudo access (only for system mode / privileged ports)

## Development

```bash
# Build
make build

# Run tests
make test

# Lint (clippy with -D warnings)
make clippy

# Format
make fmt

# Clean build artifacts
make clean

# Install locally
make install
```

## Design Philosophy

- **Layered modules** — `commands` (CLI) → `systemd` domain → `executor` seam. Keeps subprocess
  invocations behind a trait so unit tests can drive `Manager` with a `FakeSystemctl` double.
- **Typed errors** — `thiserror`-derived `Error` enum in the library; `clap`/`anyhow`-style
  surfaces stay at the binary edge only.
- **No runtime dispatch in hot paths** — concrete types everywhere except the testable
  `Box<dyn SystemctlExec>` boundary.
- **Strict lints** — `cargo clippy --all-targets -- -D warnings` is enforced.
- **Explicit dependencies** — passed via function parameters; no global state.

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.

## Author

Built with ❤️ for AI-friendly system administration
