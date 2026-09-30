# Makefile for DevPulse.

GO       ?= go
PKG      ?= ./...
BIN_DIR  ?= bin
BIN      ?= $(BIN_DIR)/devpulse

CARGO    ?= cargo
DESKTOP  ?= desktop

.PHONY: all help build test test-race test-integration lint vet tidy clean run \
	desktop desktop-run desktop-test desktop-lint

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
	@echo "  desktop           Build the Rust desktop dashboard (release)"
	@echo "  desktop-run       Run the desktop dashboard (debug build)"
	@echo "  desktop-test      Run the desktop dashboard tests"
	@echo "  desktop-lint      cargo fmt --check + clippy for the dashboard"

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

# The desktop dashboard is a separate Rust crate under desktop/. It is
# not part of `all` (the pre-commit hook), so Go-only commits don't need
# a Rust toolchain.
desktop:
	cd $(DESKTOP) && $(CARGO) build --release --locked

desktop-run:
	cd $(DESKTOP) && $(CARGO) run

desktop-test:
	cd $(DESKTOP) && $(CARGO) test

desktop-lint:
	cd $(DESKTOP) && $(CARGO) fmt --check && $(CARGO) clippy --all-targets -- -D warnings

# run loads .env if present (Unix-style: `set -a` exports every var
# sourced afterwards), then invokes the binary. Use ARGS="..." to pass
# arguments, e.g. `make run ARGS="pr fetch MilesChou/devpulse 2026-05"`.
run: build
	@set -a; [ -f .env ] && . ./.env; set +a; $(BIN) $(ARGS)

