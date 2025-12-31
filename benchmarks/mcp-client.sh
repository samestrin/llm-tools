#!/bin/bash
# MCP Client - sends a single MCP request and waits for response
# Usage: echo '<json-rpc>' | ./mcp-client.sh <mcp-server-command>

set -e

# Read the request from stdin
REQUEST=$(cat)

# Send to MCP server, capture response
# The server runs on stdio, so we pipe in/out
echo "$REQUEST" | timeout 10 "$@" 2>/dev/null | head -1
