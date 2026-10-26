BINARY_NAME=termcall
CMD_PATH=./cmd/termcall
BIN_DIR=bin

.PHONY: all build install test clean loopback devices stun help

all: build

## build: Compiles the TermCall standalone binary
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "✔ Binary built at $(BIN_DIR)/$(BINARY_NAME)"

## install: Compiles and installs termcall to $(GOPATH)/bin
install:
	go install -ldflags="-s -w" $(CMD_PATH)
	@echo "✔ Installed termcall to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

## test: Runs all unit and integration tests with race detector
test:
	go test -race -v ./...

## test-fast: Runs unit tests without race detector
test-fast:
	go test ./...

## loopback: Builds and runs local audio hardware loopback test
loopback: build
	./$(BIN_DIR)/$(BINARY_NAME) loopback

## devices: Lists detected audio input and output devices
devices: build
	./$(BIN_DIR)/$(BINARY_NAME) devices

## stun: Tests RFC 5389 STUN public address discovery
stun: build
	./$(BIN_DIR)/$(BINARY_NAME) stun

## clean: Removes build artifacts
clean:
	rm -rf $(BIN_DIR)
	@echo "✔ Cleaned build artifacts"

## help: Displays available targets
help:
	@echo "TermCall Build Commands:"
	@echo "  make build      - Build binary to bin/termcall"
	@echo "  make install    - Install binary to GOPATH/bin"
	@echo "  make test       - Run test suite with race detector"
	@echo "  make loopback   - Test microphone and speaker hardware"
	@echo "  make devices    - List audio devices"
	@echo "  make stun       - Test STUN discovery"
	@echo "  make clean      - Clean build artifacts"
