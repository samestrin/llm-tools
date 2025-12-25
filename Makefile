.PHONY: all build test lint fmt vet clean install

# Binary names
SUPPORT_BINARY=llm-support
CLARIFICATION_BINARY=llm-clarification

# Build directory
BUILD_DIR=build

all: build

build: build-support build-clarification

build-support:
	go build -o $(BUILD_DIR)/$(SUPPORT_BINARY) ./cmd/llm-support

build-clarification:
	go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY) ./cmd/llm-clarification

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
	go install ./cmd/llm-support
	go install ./cmd/llm-clarification

# Cross-platform builds
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_BINARY)-linux-amd64 ./cmd/llm-support
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(SUPPORT_BINARY)-linux-arm64 ./cmd/llm-support
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY)-linux-amd64 ./cmd/llm-clarification
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY)-linux-arm64 ./cmd/llm-clarification

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_BINARY)-darwin-amd64 ./cmd/llm-support
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(SUPPORT_BINARY)-darwin-arm64 ./cmd/llm-support
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY)-darwin-amd64 ./cmd/llm-clarification
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY)-darwin-arm64 ./cmd/llm-clarification

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(SUPPORT_BINARY)-windows-amd64.exe ./cmd/llm-support
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(CLARIFICATION_BINARY)-windows-amd64.exe ./cmd/llm-clarification
