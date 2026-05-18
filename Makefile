# Makefile for DevPulse.

GO       ?= go
PKG      ?= ./...
BIN_DIR  ?= bin
BIN      ?= $(BIN_DIR)/devpulse

.PHONY: help build test test-race test-integration lint vet tidy clean run

help:
	@echo "Targets:"
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

run: build
	$(BIN) $(ARGS)

