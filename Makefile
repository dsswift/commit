# Makefile for commit tool

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d-%H%M)
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build build-all clean test bench lint install

all: build

# Build for current platform
build:
	go build $(LDFLAGS) -o bin/commit ./cmd/commit

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/commit-linux-amd64 ./cmd/commit
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/commit-linux-arm64 ./cmd/commit

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/commit-darwin-amd64 ./cmd/commit
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/commit-darwin-arm64 ./cmd/commit

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/commit-windows-amd64.exe ./cmd/commit

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run linter
lint:
	golangci-lint run

# Install locally
install: build
	cp bin/commit ~/.local/bin/commit

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Tidy dependencies
tidy:
	go mod tidy
