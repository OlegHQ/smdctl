# Repository guidance

This repository contains `smdctl`, a Go CLI for managing systemd services on Linux. Treat the documented CLI behavior, unit-file output, file locations, and errors as compatibility contracts.

## Layout

- `cmd/smdctl` contains the minimal executable entry point and process exit handling.
- `internal/commands` owns the Cobra command tree and CLI handlers, split by command area.
- `internal/config` owns YAML configuration and defaults.
- `internal/systemd` owns service models, systemd operations, unit generation, and systemd process adapters; use `github.com/coreos/go-systemd/v22/unit` to parse unit files.
- `internal/output` owns table and structured output.
- `internal/env` owns editor and sudo process helpers.
- `internal/process` normalizes subprocess spawn errors at the OS boundary.
- Tests live beside their owning packages and cover command parsing, formats, configuration, and systemd operations.

Keep the package graph acyclic and put behavior with its owner. Keep `main` minimal and return errors to the executable boundary. Use standard library APIs first and maintained ecosystem libraries for established formats, CLI parsing, systemd units, and tables; do not hand-roll parsers, command frameworks, or tables when a suitable package exists.

## Compatibility and verification

- Preserve subcommands, flags, defaults, stdout/stderr behavior, exit status, YAML shape, generated systemd directives, and filesystem paths unless explicitly requested otherwise.
- Add focused tests for externally visible behavior and important error paths.
- Run `gofmt`, `go test ./...`, `go vet ./...`, and `go build ./cmd/smdctl` after relevant changes.
