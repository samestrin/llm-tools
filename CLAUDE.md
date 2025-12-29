# Claude Code Instructions for llm-tools

## Build

Build all binaries to the `build/` directory:

```bash
mkdir -p build && \
go build -o build/llm-support ./cmd/llm-support && \
go build -o build/llm-support-mcp ./cmd/llm-support-mcp && \
go build -o build/llm-clarification ./cmd/llm-clarification && \
go build -o build/llm-clarification-mcp ./cmd/llm-clarification-mcp && \
go build -o build/llm-filesystem ./cmd/llm-filesystem && \
go build -o build/llm-semantic ./cmd/llm-semantic
```

## Install

Copy binaries to `/usr/local/bin/`:

```bash
sudo cp build/llm-* /usr/local/bin/
```
