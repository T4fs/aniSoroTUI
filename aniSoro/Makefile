.PHONY: build run test lint clean install

BINARY_NAME=anitui
CMD_DIR=./cmd/anitui
BUILD_DIR=./build

GOOS := $(shell go env GOOS 2>/dev/null || echo unknown)
ifeq ($(GOOS),windows)
	EXT = .exe
else
	EXT =
endif

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev)
LDFLAGS = -s -w -X github.com/anitui/anitui/internal/tui.Version=$(VERSION)

build:
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)$(EXT) $(CMD_DIR)

run:
	go run $(CMD_DIR)

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

install:
	go install -ldflags="$(LDFLAGS)" $(CMD_DIR)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

build-windows-arm64:
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_DIR)

build-all: build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64
