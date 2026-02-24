APP_NAME  := keenetic-tray
BUILD_DIR := bin
VERSION   ?= dev

LDFLAGS_BASE := -s -w -X main.Version=$(VERSION)
LDFLAGS_WIN  := $(LDFLAGS_BASE) -H windowsgui
LDFLAGS_UNX  := $(LDFLAGS_BASE)

.PHONY: all windows linux mac run clean

## Default target — builds for the current OS
all: windows

## Build for Windows (run on Windows or via CI)
windows:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS_WIN)" -o $(BUILD_DIR)/$(APP_NAME).exe .

## Build for Linux (run on Linux or via CI)
linux:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS_UNX)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 .

## Build for macOS arm64 (run on macOS or via CI)
mac-arm:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build -ldflags="$(LDFLAGS_UNX)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 .

## Build for macOS x86_64 (run on macOS or via CI)
mac-amd:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS_UNX)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 .

## Build for both macOS architectures
mac: mac-arm mac-amd

## Run locally without building a binary
run:
	CGO_ENABLED=1 go run .

## Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
