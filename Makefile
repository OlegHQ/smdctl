.PHONY: build install test clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION)

build:
	@echo "Building smdctl..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/smdctl .
	@echo "Build complete: bin/smdctl"

install: build
	@echo "Installing smdctl..."
	sudo install -m 755 bin/smdctl /usr/local/bin/smdctl
	sudo mkdir -p /etc/smdctl/env
	@echo "smdctl installed to /usr/local/bin/smdctl"
	@echo ""
	@echo "Run 'smdctl help' to get started"

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

.DEFAULT_GOAL := build
