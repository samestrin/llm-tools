# Claude Code Instructions for llm-tools

## Important: llm-filesystem is a Port

`llm-filesystem` is a port of an existing project. **Do not change existing behavior** - only add new features. The MCP wrapper (`llm-filesystem-mcp`) should only add `--json`, not `--min`, to preserve compatibility with the original project's output format.

## Build

Build all binaries to the `build/` directory:

```bash
mkdir -p build && \
go build -o build/llm-support ./cmd/llm-support && \
go build -o build/llm-support-mcp ./cmd/llm-support-mcp && \
go build -o build/llm-clarification ./cmd/llm-clarification && \
go build -o build/llm-clarification-mcp ./cmd/llm-clarification-mcp && \
go build -o build/llm-filesystem ./cmd/llm-filesystem && \
go build -o build/llm-filesystem-mcp ./cmd/llm-filesystem-mcp && \
go build -o build/llm-semantic ./cmd/llm-semantic && \
go build -o build/llm-semantic-mcp ./cmd/llm-semantic-mcp
```

## Install

Copy binaries to `/usr/local/bin/`:

```bash
sudo cp build/llm-* /usr/local/bin/
```
