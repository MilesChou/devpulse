# Makefile for DevPulse.

GO       ?= go
PKG      ?= ./...
BIN_DIR  ?= bin
BIN      ?= $(BIN_DIR)/devpulse

.PHONY: all help build test test-race test-integration lint vet tidy clean run

# Default target — used by the pre-commit hook.
all: lint test build

help:
	@echo "Targets:"
	@echo "  all               (default) lint + test + build"
	@echo "  build             Build the devpulse binary into $(BIN)"
	@echo "  test              Run unit tests"
	@echo "  test-race         Run unit tests with -race"
	@echo "  test-integration  Run integration tests (requires Docker)"
	@echo "  lint              Run go vet + gofmt check"
	@echo "  tidy              go mod tidy"
	@echo "  clean             Remove build artifacts"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/devpulse

test:
	$(GO) test -count=1 $(PKG)

test-race:
	$(GO) test -race -count=1 $(PKG)

test-integration:
	$(GO) test -race -count=1 -tags=integration $(PKG)

lint: vet
	@gofmt -l . | grep -v '^vendor/' | tee /dev/stderr | (! read)

vet:
	$(GO) vet $(PKG)

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)

# run loads .env if present (Unix-style: `set -a` exports every var
# sourced afterwards), then invokes the binary. Use ARGS="..." to pass
# arguments, e.g. `make run ARGS="pr fetch MilesChou/devpulse 2026-05"`.
run: build
	@set -a; [ -f .env ] && . ./.env; set +a; $(BIN) $(ARGS)

