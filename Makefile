SHELL := /usr/bin/env bash

BINARY ?= hatch
BIN_DIR ?= bin
GO ?= go
GOFLAGS ?=
GO_VERSION_MIN ?= 1.24
IMAGE ?= hatch:local

.PHONY: help setup deps tidy test vet build image docker-build e2e clean

help:
	@printf 'Targets:\n'
	@printf '  setup        Check local development prerequisites\n'
	@printf '  deps         Download Go module dependencies\n'
	@printf '  tidy         Normalize go.mod and go.sum\n'
	@printf '  test         Run Go tests\n'
	@printf '  vet          Run go vet\n'
	@printf '  build        Build $(IMAGE), then build the Hatch Go CLI into $(BIN_DIR)/$(BINARY)\n'
	@printf '  image        Build the Hatch container image\n'
	@printf '  docker-build Alias for image\n'
	@printf '  e2e          Run the Guacamole smoke test\n'
	@printf '  clean        Remove local build outputs\n'

setup:
	@command -v "$(GO)" >/dev/null || { echo "ERROR: Go $(GO_VERSION_MIN)+ is required."; exit 1; }
	@command -v docker >/dev/null || { echo "ERROR: Docker is required."; exit 1; }
	@command -v curl >/dev/null || { echo "ERROR: curl is required."; exit 1; }
	@command -v nc >/dev/null || { echo "ERROR: nc is required."; exit 1; }
	@command -v node >/dev/null || { echo "ERROR: Node.js is required for E2E tests."; exit 1; }
	@command -v npm >/dev/null || { echo "ERROR: npm is required for E2E tests."; exit 1; }
	@"$(GO)" version | awk -v min="$(GO_VERSION_MIN)" '{ split($$3, v, "go"); split(v[2], parts, "."); split(min, want, "."); if ((parts[1] + 0) < (want[1] + 0) || ((parts[1] + 0) == (want[1] + 0) && (parts[2] + 0) < (want[2] + 0))) { printf "ERROR: Go %s+ is required, found %s.\n", min, $$3; exit 1 } }'
	@docker info >/dev/null || { echo "ERROR: Docker is installed but the daemon is not reachable."; exit 1; }
	@echo "Local development prerequisites are available."

deps:
	$(GO) mod download

tidy:
	$(GO) mod tidy

test: deps
	$(GO) test $(GOFLAGS) ./...

vet: deps
	$(GO) vet $(GOFLAGS) ./...

build: deps image
	mkdir -p "$(BIN_DIR)"
	$(GO) build $(GOFLAGS) -o "$(BIN_DIR)/$(BINARY)" ./cmd/hatch

image:
	docker build -t "$(IMAGE)" .

docker-build: image

e2e: setup
	scripts/e2e-guacamole.sh

clean:
	rm -rf "$(BIN_DIR)"
