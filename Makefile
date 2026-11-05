# ==============================================================================
# nrf52840-hsm-cli (gphsm) Makefile
# Cross-platform compilation for macOS, Linux, and Windows
# ==============================================================================

SHELL := /bin/bash
BINARY_NAME := gphsm
BIN_DIR := bin
SRC_PKG := ./cmd/gphsm

.PHONY: help all build build-all test clean

help:
	@echo "=================================================================="
	@echo "                    gphsm CLI Build Automation                    "
	@echo "=================================================================="
	@echo "Targets:"
	@echo "  make build          Build '$(BINARY_NAME)' (static binary, CGO_ENABLED=0)"
	@echo "  make build-all      Cross-compile for darwin/linux/windows (arm64 & amd64)"
	@echo "  make test           Run unit tests"
	@echo "  make clean          Remove compiled binaries and symlinks"
	@echo "=================================================================="

all: test build

build:
	@mkdir -p $(BIN_DIR)
	@echo "[INFO] Building $(BINARY_NAME) (pure static binary, CGO_ENABLED=0)..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_PKG)
	@ln -sf $(BINARY_NAME) $(BIN_DIR)/$(BINARY_NAME)s
	@ln -sf $(BINARY_NAME) $(BIN_DIR)/hsmutil
	@ln -sf $(BINARY_NAME) $(BIN_DIR)/hsmutils
	@echo "[SUCCESS] Host binary generated: $(BIN_DIR)/$(BINARY_NAME) (symlinks: $(BINARY_NAME)s, hsmutil, hsmutils)"

build-all:
	@mkdir -p $(BIN_DIR)
	@echo "[INFO] Cross-compiling $(BINARY_NAME) across 5 target architectures (CGO_ENABLED=0)..."
	
	@echo "  -> darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(SRC_PKG)
	
	@echo "  -> linux/arm64..."
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(SRC_PKG)
	
	@echo "  -> linux/amd64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(SRC_PKG)
	
	@echo "  -> windows/arm64..."
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-windows-arm64.exe $(SRC_PKG)
	
	@echo "  -> windows/amd64..."
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(SRC_PKG)

	@echo "[SUCCESS] Cross-compilation complete in $(BIN_DIR)/:"
	@ls -lh $(BIN_DIR)

test:
	@echo "[INFO] Running tests..."
	go test -v ./...

clean:
	@echo "[INFO] Removing binary directory..."
	@rm -rf $(BIN_DIR)
