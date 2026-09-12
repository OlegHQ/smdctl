.PHONY: build install test vet fmt clean

# Release binary (Linux + systemd target).
build:
	@mkdir -p target/release
	go build -trimpath -ldflags="-s -w" -o target/release/smdctl ./cmd/smdctl
	@echo "Built: target/release/smdctl"

install: build
	sudo install -m 755 target/release/smdctl /usr/local/bin/smdctl
	sudo mkdir -p /etc/smdctl/env
	@echo "smdctl installed to /usr/local/bin/smdctl"
	@echo ""
	@echo "Run 'smdctl help' to get started"

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

clean:
	rm -rf target

.DEFAULT_GOAL := build
