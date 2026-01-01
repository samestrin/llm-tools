#!/bin/bash
# Benchmark: llm-filesystem (Go) vs fast-filesystem-mcp (TypeScript)
#
# Measures:
# 1. Cold start time (time to first output)
# 2. MCP handshake + tool list
# 3. File read operation
# 4. Directory tree operation
#
# Usage: ./benchmark.sh [warmup_runs] [benchmark_runs]

set -e

WARMUP=${1:-3}
RUNS=${2:-10}
TEST_DIR="$HOME/Documents/GitHub/llm-interface"
BENCHMARK_DIR="$(dirname "$0")"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo -e "${BLUE}=== MCP Filesystem Benchmark ===${NC}"
echo "Test directory: $TEST_DIR"
echo "Files in test dir: $(find "$TEST_DIR" -type f 2>/dev/null | wc -l | tr -d ' ')"
echo "Warmup runs: $WARMUP"
echo "Benchmark runs: $RUNS"
echo ""

# Check tools
for cmd in llm-filesystem fast-filesystem-mcp hyperfine python3; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: $cmd not found"
        exit 1
    fi
done

# Versions
echo -e "${GREEN}Versions:${NC}"
echo "  llm-filesystem: $(llm-filesystem --version 2>&1 | head -1)"
echo "  fast-filesystem-mcp: $(fast-filesystem-mcp --version 2>&1 | head -1)"
echo ""

# ============================================================================
# Create Python MCP test clients
# ============================================================================

# Generic MCP client that sends requests and reads responses
cat > /tmp/mcp-bench-client.py << 'PYSCRIPT'
#!/usr/bin/env python3
"""MCP Benchmark Client - sends requests and measures response time."""
import subprocess
import json
import sys
import threading
import time

def run_mcp_test(server_cmd, requests, timeout=5):
    """Run MCP server, send requests, count responses."""
    proc = subprocess.Popen(
        server_cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        bufsize=1
    )

    responses = []

    def reader():
        while len(responses) < len(requests):
            try:
                line = proc.stdout.readline()
                if line.strip():
                    responses.append(json.loads(line))
            except:
                break

    t = threading.Thread(target=reader, daemon=True)
    t.start()

    # Send all requests
    for req in requests:
        proc.stdin.write(json.dumps(req) + "\n")
        proc.stdin.flush()

    # Wait for responses
    t.join(timeout=timeout)
    proc.terminate()

    return len(responses)

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", required=True, choices=["go", "ts"])
    parser.add_argument("--test", required=True, choices=["handshake", "read", "tree"])
    parser.add_argument("--dir", default="/tmp")
    args = parser.parse_args()

    # Server command
    if args.server == "go":
        server_cmd = ["llm-filesystem", "--allowed-dirs", args.dir]
    else:
        server_cmd = ["fast-filesystem-mcp"]

    # Build requests based on test type
    init_req = {
        "jsonrpc": "2.0", "id": 1, "method": "initialize",
        "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                   "clientInfo": {"name": "bench", "version": "1.0"}}
    }

    if args.test == "handshake":
        requests = [
            init_req,
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
        ]
    elif args.test == "read":
        tool_name = "fast_read_file" if args.server == "go" else "read_file"
        requests = [
            init_req,
            {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
             "params": {"name": tool_name, "arguments": {"path": f"{args.dir}/package.json"}}}
        ]
    elif args.test == "tree":
        tool_name = "fast_get_directory_tree" if args.server == "go" else "get_directory_tree"
        requests = [
            init_req,
            {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
             "params": {"name": tool_name, "arguments": {"path": args.dir, "depth": 3}}}
        ]

    count = run_mcp_test(server_cmd, requests)
    print(f"Responses: {count}/{len(requests)}")
    sys.exit(0 if count == len(requests) else 1)
PYSCRIPT
chmod +x /tmp/mcp-bench-client.py

# ============================================================================
# Benchmark 1: Cold Start Time
# ============================================================================
echo -e "${GREEN}Benchmark 1: Cold Start Time${NC}"
echo "Measures time for server to start and output banner."
echo ""

hyperfine \
    --warmup "$WARMUP" \
    --runs "$RUNS" \
    --export-json "$BENCHMARK_DIR/results-coldstart.json" \
    --command-name "llm-filesystem (Go)" \
    "llm-filesystem --allowed-dirs '$TEST_DIR' < /dev/null 2>&1 | head -1" \
    --command-name "fast-filesystem-mcp (TS)" \
    "fast-filesystem-mcp < /dev/null 2>&1 | head -1"

echo ""

# ============================================================================
# Benchmark 2: MCP Handshake
# ============================================================================
echo -e "${GREEN}Benchmark 2: MCP Initialize + Tools List${NC}"
echo "Measures full MCP handshake including tool registration."
echo ""

hyperfine \
    --warmup "$WARMUP" \
    --runs "$RUNS" \
    --export-json "$BENCHMARK_DIR/results-handshake.json" \
    --command-name "llm-filesystem (Go)" \
    "python3 /tmp/mcp-bench-client.py --server go --test handshake --dir '$TEST_DIR'" \
    --command-name "fast-filesystem-mcp (TS)" \
    "python3 /tmp/mcp-bench-client.py --server ts --test handshake --dir '$TEST_DIR'"

echo ""

# ============================================================================
# Benchmark 3: File Read
# ============================================================================
echo -e "${GREEN}Benchmark 3: Read File via MCP${NC}"
echo "Measures init + read package.json."
echo ""

hyperfine \
    --warmup "$WARMUP" \
    --runs "$RUNS" \
    --export-json "$BENCHMARK_DIR/results-read.json" \
    --command-name "llm-filesystem (Go)" \
    "python3 /tmp/mcp-bench-client.py --server go --test read --dir '$TEST_DIR'" \
    --command-name "fast-filesystem-mcp (TS)" \
    "python3 /tmp/mcp-bench-client.py --server ts --test read --dir '$TEST_DIR'"

echo ""

# ============================================================================
# Benchmark 4: Directory Tree
# ============================================================================
echo -e "${GREEN}Benchmark 4: Directory Tree (depth 3)${NC}"
echo "Measures init + get directory tree."
echo ""

hyperfine \
    --warmup "$WARMUP" \
    --runs "$RUNS" \
    --export-json "$BENCHMARK_DIR/results-tree.json" \
    --command-name "llm-filesystem (Go)" \
    "python3 /tmp/mcp-bench-client.py --server go --test tree --dir '$TEST_DIR'" \
    --command-name "fast-filesystem-mcp (TS)" \
    "python3 /tmp/mcp-bench-client.py --server ts --test tree --dir '$TEST_DIR'"

echo ""

# ============================================================================
# Summary
# ============================================================================
echo -e "${BLUE}=== Benchmark Complete ===${NC}"
echo ""
echo "Results saved to:"
ls -1 "$BENCHMARK_DIR"/results-*.json 2>/dev/null | while read f; do echo "  - $f"; done
echo ""
echo -e "${YELLOW}Quick Summary:${NC}"

# Parse and display results
python3 << PYSUMMARY
import json
import os
import glob

benchmark_dir = "$BENCHMARK_DIR"
results = {}

for f in sorted(glob.glob(os.path.join(benchmark_dir, "results-*.json"))):
    name = os.path.basename(f).replace("results-", "").replace(".json", "")
    with open(f) as fp:
        data = json.load(fp)

    go_time = ts_time = None
    for r in data["results"]:
        if "Go" in r["command"]:
            go_time = r["mean"] * 1000
            go_std = r["stddev"] * 1000
        else:
            ts_time = r["mean"] * 1000
            ts_std = r["stddev"] * 1000

    if go_time and ts_time:
        speedup = ts_time / go_time
        print(f"\n{name}:")
        print(f"  llm-filesystem (Go):    {go_time:7.1f}ms ± {go_std:.1f}ms")
        print(f"  fast-filesystem (TS):   {ts_time:7.1f}ms ± {ts_std:.1f}ms")
        print(f"  Go is {speedup:.1f}x faster")
PYSUMMARY

# Cleanup
rm -f /tmp/mcp-bench-client.py

echo ""
echo "Done!"
