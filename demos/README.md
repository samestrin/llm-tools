# Performance Demos

VHS tape files for generating terminal GIFs that demonstrate the performance difference between the Python and Go versions of llm-support.

## Prerequisites

1. **VHS** - Terminal recording tool
   ```bash
   brew install vhs
   ```

2. **Go binaries installed**
   ```bash
   sudo cp build/llm-support /usr/local/bin/
   ```

3. **Python script symlink** (for clean demo output)
   ```bash
   # Point to your Python llm-support.py location
   ln -sf /path/to/llm-support.py demos/llm-support.py
   ```

4. **Test repository** - The demos use [llm-interface](https://github.com/samestrin/llm-interface) as a large test repo (21k+ files)

## Running Demos

```bash
# Generate the performance comparison GIF
vhs demos/performance.tape

# Output: demos/performance.gif
```

## Performance Results

Tested on llm-interface repository (21,322 files):

| Command    | Python   | Go      | Speedup |
|------------|----------|---------|---------|
| multigrep  | ~500ms   | ~27ms   | **18x** |
| detect     | ~60ms    | ~6ms    | **10x** |
| tree       | ~84ms    | ~22ms   | **4x**  |

The Go version achieves significant speedups through:
- Native compilation (no interpreter startup)
- Goroutines for parallel file operations
- Efficient file system traversal
