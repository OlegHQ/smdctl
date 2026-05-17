.PHONY: build install test clippy fmt clean

# Release binary (Linux + systemd target).
build:
	cargo build --release
	@echo "Built: target/release/smdctl"

install: build
	sudo install -m 755 target/release/smdctl /usr/local/bin/smdctl
	sudo mkdir -p /etc/smdctl/env
	@echo "smdctl installed to /usr/local/bin/smdctl"
	@echo ""
	@echo "Run 'smdctl help' to get started"

test:
	cargo test

clippy:
	cargo clippy --all-targets -- -D warnings

fmt:
	cargo fmt

clean:
	cargo clean

.DEFAULT_GOAL := build
