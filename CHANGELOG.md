# Changelog

All notable changes to `smdctl` are documented here. This project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- MIT license file, included in release archives.

## [0.1.0] - 2026-09-12

### Added

- Go implementation of the systemd service CLI and its existing command set.
- Linux amd64 and arm64 release archives with SHA-256 checksums.
- A checksum-verifying Linux installer that writes to `$HOME/.local/bin` without editing PATH or shell profiles.
- GitHub Actions CI and GoReleaser-based GitHub releases.
