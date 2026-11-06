# ==============================================================================
# nrf52840-hsm-cli (gphsm) Makefile
# Cross-platform compilation for macOS, Linux, and Windows
# ==============================================================================

SHELL := /bin/bash
BINARY_NAME := gphsm
BIN_DIR := bin
SRC_PKG := ./cmd/gphsm

UNAME_S := $(shell uname -s)

.PHONY: help all build build-all test clean

help:
	@echo "=================================================================="
	@echo "                    gphsm CLI Build Automation                    "
	@echo "=================================================================="
	@echo "Targets:"
	@echo "  make build          Build '$(BINARY_NAME)' (CGO_ENABLED=1 on macOS, CGO_ENABLED=0 on Linux/Windows)"
	@echo "  make build-all      Cross-compile for darwin/linux/windows (arm64 & amd64)"
	@echo "  make test           Run unit tests"
	@echo "  make clean          Remove compiled binaries"
	@echo "=================================================================="

all: test build

build:
	@mkdir -p $(BIN_DIR)
ifeq ($(UNAME_S),Darwin)
	@echo "[INFO] Building $(BINARY_NAME) for macOS (Darwin, CGO_ENABLED=1)..."
	CGO_ENABLED=1 go build -ldflags="-w" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_PKG)
else
	@echo "[INFO] Building $(BINARY_NAME) (pure static binary, CGO_ENABLED=0)..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_PKG)
endif
	@echo "[SUCCESS] Host binary generated: $(BIN_DIR)/$(BINARY_NAME)"

build-all:
	@mkdir -p $(BIN_DIR)
	@echo "[INFO] Cross-compiling $(BINARY_NAME) across 5 target architectures..."
	
	@echo "  -> darwin/arm64 (macOS dynamic linking, CGO_ENABLED=1)..."
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-w" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(SRC_PKG)
	
	@echo "  -> linux/arm64 (pure static binary, CGO_ENABLED=0)..."
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(SRC_PKG)
	
	@echo "  -> linux/amd64 (pure static binary, CGO_ENABLED=0)..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC_PKG)
	
	@echo "  -> windows/arm64 (pure static binary, CGO_ENABLED=0)..."
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-windows-arm64.exe $(SRC_PKG)
	
	@echo "  -> windows/amd64 (pure static binary, CGO_ENABLED=0)..."
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC_PKG)

	@echo "[SUCCESS] Cross-compilation complete in $(BIN_DIR)/:"
	@ls -lh $(BIN_DIR)

test:
	@echo "[INFO] Running tests..."
	go test -v ./...

clean:
	@echo "[INFO] Removing binary directory..."
	@rm -rf $(BIN_DIR)
