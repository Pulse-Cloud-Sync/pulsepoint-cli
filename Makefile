# PulsePoint Makefile
# Build automation for the PulsePoint cloud sync CLI tool

# Variables
BINARY_NAME=pulsepoint
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}"
GO_FILES=$(shell find . -name '*.go' -type f)

# Go related variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard *.go)

# Use richgo for colored test output if available
GOTEST=$(shell which richgo >/dev/null 2>&1 && echo "richgo test" || echo "go test")

# Build the binary for the current platform
.PHONY: build
build:
	@echo "Building PulsePoint ${VERSION}..."
	@go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME} ./cmd/pulsepoint

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME}-linux-amd64 ./cmd/pulsepoint
	@GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME}-linux-arm64 ./cmd/pulsepoint

.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME}-darwin-amd64 ./cmd/pulsepoint
	@GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME}-darwin-arm64 ./cmd/pulsepoint

.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${GOBIN}/${BINARY_NAME}-windows-amd64.exe ./cmd/pulsepoint

# Run the application
.PHONY: run
run: build
	@echo "Running PulsePoint..."
	@${GOBIN}/${BINARY_NAME}

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@${GOTEST} -v -race -coverprofile=coverage.out ./...

# Run unit tests only
.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@${GOTEST} -v -short ./...

# Run integration tests only
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	@${GOTEST} -v -run Integration ./test/integration

# Run tests with coverage report
.PHONY: test-coverage
test-coverage: test
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Install the binary
.PHONY: install
install: build
	@echo "Installing PulsePoint..."
	@cp ${GOBIN}/${BINARY_NAME} ${GOPATH}/bin/
	@echo "PulsePoint installed to ${GOPATH}/bin/${BINARY_NAME}"

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "Uninstalling PulsePoint..."
	@rm -f ${GOPATH}/bin/${BINARY_NAME}
	@echo "PulsePoint uninstalled"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@go clean
	@rm -rf ${GOBIN}
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run linters
.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w ${GO_FILES}

# Run go mod tidy
.PHONY: tidy
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# Check for security vulnerabilities
.PHONY: security
security:
	@echo "Checking for vulnerabilities..."
	@govulncheck ./...

# Generate mocks for testing
.PHONY: mocks
mocks:
	@echo "Generating mocks..."
	@mockgen -source=internal/core/interfaces.go -destination=internal/mocks/mocks.go

# Development setup
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/kyoh86/richgo@latest
	@echo "Development environment ready!"

# Docker build
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t pulsepoint:${VERSION} .

# Docker run
.PHONY: docker-run
docker-run: docker-build
	@echo "Running PulsePoint in Docker..."
	@docker run --rm -it -v ${HOME}/.pulsepoint:/root/.pulsepoint pulsepoint:${VERSION}

# Help command
.PHONY: help
help:
	@echo "PulsePoint Makefile Commands:"
	@echo ""
	@echo "  make build        - Build PulsePoint for current platform"
	@echo "  make build-all    - Build for all platforms (Linux, macOS, Windows)"
	@echo "  make run          - Build and run PulsePoint"
	@echo "  make test         - Run all tests"
	@echo "  make test-unit    - Run unit tests only"
	@echo "  make test-integration - Run integration tests only"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make bench        - Run benchmarks"
	@echo "  make install      - Install PulsePoint to GOPATH/bin"
	@echo "  make uninstall    - Uninstall PulsePoint"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make lint         - Run linters"
	@echo "  make fmt          - Format code"
	@echo "  make tidy         - Tidy go modules"
	@echo "  make security     - Check for vulnerabilities"
	@echo "  make dev-setup    - Install development tools"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Run in Docker container"
	@echo "  make help         - Show this help message"

# Default target
.DEFAULT_GOAL := help
