.PHONY: all build test lint fmt vet clean install build-all release

# Binary names
SUPPORT_BINARY=llm-support
SUPPORT_MCP_BINARY=llm-support-mcp

# Build directory
BUILD_DIR=build

# Version from git
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/samestrin/llm-tools/internal/support/commands.Version=$(VERSION)"

all: build

build: build-support build-mcp

build-support:
	@echo "Building $(SUPPORT_BINARY)..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY) ./cmd/llm-support

build-mcp:
	@echo "Building MCP servers..."
	go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY) ./cmd/llm-support-mcp

test:
	go test -v ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover ./...

lint: vet fmt

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf $(BUILD_DIR)
	go clean

install:
	go install $(LDFLAGS) ./cmd/llm-support
	go install ./cmd/llm-support-mcp

# Cross-platform builds
build-all: clean
	@mkdir -p $(BUILD_DIR)
	@echo "🍎 Building for macOS (ARM64)..."
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY)-darwin-arm64 ./cmd/llm-support
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY)-darwin-arm64 ./cmd/llm-support-mcp
	@echo "🍎 Building for macOS (AMD64)..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY)-darwin-amd64 ./cmd/llm-support
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY)-darwin-amd64 ./cmd/llm-support-mcp
	@echo "🐧 Building for Linux (AMD64)..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY)-linux-amd64 ./cmd/llm-support
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY)-linux-amd64 ./cmd/llm-support-mcp
	@echo "🐧 Building for Linux (ARM64)..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY)-linux-arm64 ./cmd/llm-support
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY)-linux-arm64 ./cmd/llm-support-mcp
	@echo "🪟 Building for Windows (AMD64)..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(SUPPORT_BINARY)-windows-amd64.exe ./cmd/llm-support
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_MCP_BINARY)-windows-amd64.exe ./cmd/llm-support-mcp
	@echo "✅ Build complete!"

# Package releases
release: build-all
	@echo "📦 Packaging releases..."
	cd $(BUILD_DIR) && tar czf llm-tools-darwin-arm64.tar.gz *-darwin-arm64
	cd $(BUILD_DIR) && tar czf llm-tools-darwin-amd64.tar.gz *-darwin-amd64
	cd $(BUILD_DIR) && tar czf llm-tools-linux-amd64.tar.gz *-linux-amd64
	cd $(BUILD_DIR) && tar czf llm-tools-linux-arm64.tar.gz *-linux-arm64
	cd $(BUILD_DIR) && zip -q llm-tools-windows-amd64.zip *-windows-amd64.exe
	@echo "📋 Generating checksums..."
	cd $(BUILD_DIR) && shasum -a 256 *.tar.gz *.zip > checksums.txt
	@echo "✅ Release artifacts in ./build/"
	@ls -lh $(BUILD_DIR)/*.tar.gz $(BUILD_DIR)/*.zip $(BUILD_DIR)/checksums.txt
