# Application Name Base
BINARY_NAME := go-hdr
GO_SRC := ./cmd/go-hdr/...

# Output directory for binaries
BUILD_DIR := ./bin

# Windows Target Options
WIN_OUT := $(BUILD_DIR)/$(BINARY_NAME).exe
WIN_GOOS := windows
WIN_GOARCH := amd64
WIN_CGO := 1
WIN_CC := x86_64-w64-mingw32-gcc

# Linux Target Options
LINUX_OUT := $(BUILD_DIR)/$(BINARY_NAME)
LINUX_GOOS := linux
LINUX_GOARCH := amd64
LINUX_CGO := 1
LINUX_CC := gcc

# Linker & Verbosity Flags:
# -v : Prints names of packages as they compile
# -x : Prints exact command invocation details executed under the hood
# -ldflags "-v" : Prints detailed internal linker stages
GO_VERBOSE_FLAGS := -v -x

WIN_LDFLAGS := -ldflags="-v -H windowsgui -s -w"
LINUX_LDFLAGS := -ldflags="-v -s -w"

.PHONY: all setup build-win build-linux clean help

# Default target builds both binaries into ./bin/
all: build-win build-linux

## setup: Installs required Linux cross-compilers and Go dependencies
setup:
	@echo "==> [VERBOSE] Installing MinGW cross-compiler and Linux build tools..."
	sudo apt update && sudo apt install -y gcc-mingw-w64-x86-64 gcc libgl1-mesa-dev xorg-dev
	@echo "==> [VERBOSE] Downloading and verifying Go module dependencies..."
	go mod tidy -v

## build-win: Cross-compiles the Windows .exe with verbose command logging
build-win:
	@mkdir -p $(BUILD_DIR)
	@echo "==> [VERBOSE] Building Windows (x64) binary -> $(WIN_OUT)"
	GOOS=$(WIN_GOOS) \
	GOARCH=$(WIN_GOARCH) \
	CGO_ENABLED=$(WIN_CGO) \
	CC="$(WIN_CC)" \
	go build $(GO_VERBOSE_FLAGS) $(WIN_LDFLAGS) -o $(WIN_OUT) $(GO_SRC)
	@echo "==> [VERBOSE] Success! Output: $(WIN_OUT)"

## build-linux: Builds native Linux binary with verbose command logging
build-linux:
	@mkdir -p $(BUILD_DIR)
	@echo "==> [VERBOSE] Building Linux (x64) binary -> $(LINUX_OUT)"
	GOOS=$(LINUX_GOOS) \
	GOARCH=$(LINUX_GOARCH) \
	CGO_ENABLED=$(LINUX_CGO) \
	CC="$(LINUX_CC)" \
	go build $(GO_VERBOSE_FLAGS) $(LINUX_LDFLAGS) -o $(LINUX_OUT) $(GO_SRC)
	@echo "==> [VERBOSE] Success! Output: $(LINUX_OUT)"

## run: Cross-compiles and runs the executable
run-win: build-win
	@echo "==> Launching $(WIN_OUT) in foreground..."
	$(WIN_OUT) | cat

## clean: Deletes all compiled binaries and build artifacts with file details
clean:
	@echo "==> [VERBOSE] Cleaning build artifacts..."
	rm -rvf $(BUILD_DIR)

## help: Shows available Makefile commands
help:
	@echo "Available commands:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'
