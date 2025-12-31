Benchmark Results: llm-filesystem (Go) vs fast-filesystem-mcp (TypeScript)

| Benchmark      | Go     | TypeScript | Speedup      |
|----------------|--------|------------|--------------|
| Cold Start     | 5.2ms  | 85.1ms     | 16.5x faster |
| MCP Handshake  | 40.8ms | 110.4ms    | 2.7x faster  |
| File Read      | 49.5ms | 108.2ms    | 2.2x faster  |
| Directory Tree | 50.9ms | 113.7ms    | 2.2x faster  |

The biggest win is cold start time - Go is ~16x faster because it's a compiled binary vs Node.js JIT startup. For actual MCP operations, Go is consistently 2-3x faster.

Usage

# Run full benchmark (default: 3 warmup, 10 runs)
cd benchmarks && ./benchmark.sh

# Quick benchmark
./benchmark.sh 2 5

# Custom runs
./benchmark.sh <warmup> <runs>

The results are saved as JSON in benchmarks/results-*.json for further analysis.