# llm-support - Quick Reference

Fast lookup for all commands with examples.

## Command Summary

| # | Command | Description | Example |
|---|---------|-------------|---------|
| 1 | `listdir` | List directory with metadata | `llm-support listdir --path src/ --sizes --dates` |
| 2 | `tree` | Display directory tree | `llm-support tree --path src/ --depth 3` |
| 3 | `catfiles` | Concatenate files with headers | `llm-support catfiles src/ --max-size 5` |
| 4 | `hash` | Generate checksums | `llm-support hash file.txt -a sha256` |
| 5 | `stats` | Show file statistics | `llm-support stats --path ./project` |
| 6 | `grep` | Search files with regex | `llm-support grep "TODO" src/ -i -n` |
| 7 | `multigrep` | Search multiple keywords | `llm-support multigrep --path src/ --keywords "fn1,fn2"` |
| 8 | `multiexists` | Check if files exist | `llm-support multiexists config.json README.md` |
| 9 | `detect` | Detect project type | `llm-support detect --path . --json` |
| 10 | `discover-tests` | Find test patterns | `llm-support discover-tests --path ./project` |
| 11 | `analyze-deps` | Extract file dependencies | `llm-support analyze-deps plan.md` |
| 12 | `partition-work` | Group tasks by conflicts | `llm-support partition-work tasks.md` |
| 13 | `json` | JSON operations | `llm-support json query data.json ".name"` |
| 14 | `toml` | TOML operations | `llm-support toml get config.toml key` |
| 15 | `markdown` | Markdown processing | `llm-support markdown toc README.md` |
| 16 | `extract` | Extract patterns | `llm-support extract urls file.txt` |
| 17 | `transform` | Text transformations | `llm-support transform upper file.txt` |
| 18 | `count` | Count lines/checkboxes | `llm-support count --mode checkboxes --path plan.md` |
| 19 | `encode` | Encode text | `llm-support encode "hello" -e base64` |
| 20 | `decode` | Decode text | `llm-support decode "aGVsbG8=" -e base64` |
| 21 | `math` | Evaluate expressions | `llm-support math "2**10 + 5"` |
| 22 | `template` | Variable substitution | `llm-support template file.txt --var x=y` |
| 23 | `validate` | Validate files | `llm-support validate file.json` |
| 24 | `validate-plan` | Validate sprint plans | `llm-support validate-plan --path ./sprint-01/` |
| 25 | `diff` | Compare files | `llm-support diff file1 file2` |
| 26 | `prompt` | Execute LLM prompts | `llm-support prompt --prompt "Explain this"` |
| 27 | `foreach` | Batch process with LLM | `llm-support foreach --path src/ --template p.txt` |
| 28 | `extract-relevant` | Extract relevant content | `llm-support extract-relevant --path docs/` |
| 29 | `summarize-dir` | Summarize directory | `llm-support summarize-dir --path src/` |
| 30 | `git-context` | Get git context | `llm-support git-context` |
| 31 | `git-changes` | Count git working tree changes | `llm-support git-changes --path .planning/` |
| 32 | `repo-root` | Find git repo root | `llm-support repo-root --validate` |
| 33 | `report` | Generate reports | `llm-support report --template report.md` |
| 34 | `deps` | Show dependencies | `llm-support deps package.json` |
| 35 | `highest` | Find highest numbered dir/file | `llm-support highest --path .planning/plans` |
| 36 | `init-temp` | Initialize temp directory | `llm-support init-temp --name mysession` |
| 37 | `plan-type` | Extract plan type | `llm-support plan-type --path .planning/plans/1.0/` |
| 38 | `context` | Manage prompt variables | `llm-support context set --dir /tmp MY_VAR "value"` |
| 39 | `args` | Parse arguments | `llm-support args @path/to/plan/` |

---

## File Operations

### listdir - List Directory

```bash
# Basic listing
llm-support listdir --path src/

# With file sizes
llm-support listdir --path src/ --sizes

# With modification dates
llm-support listdir --path src/ --dates

# Both sizes and dates
llm-support listdir --path src/ --sizes --dates

# Include gitignored files
llm-support listdir --path src/ --no-gitignore
```

### tree - Directory Tree

```bash
# Basic tree
llm-support tree --path src/

# Limit depth
llm-support tree --path src/ --depth 3

# Show file sizes
llm-support tree --path src/ --sizes

# Include gitignored files
llm-support tree --path . --no-gitignore
```

### catfiles - Concatenate Files

```bash
# Concatenate directory contents
llm-support catfiles src/

# Limit total size to 5MB
llm-support catfiles src/ --max-size 5

# Multiple paths
llm-support catfiles src/ lib/ tests/
```

### hash - Generate Checksums

```bash
# SHA256 (default)
llm-support hash file.txt

# MD5
llm-support hash file.txt -a md5

# SHA1
llm-support hash file.txt -a sha1

# Multiple files
llm-support hash file1.txt file2.txt

# All Go files
llm-support hash internal/**/*.go -a sha256
```

### stats - Directory Statistics

```bash
# Basic stats
llm-support stats --path ./project

# Include gitignored files
llm-support stats --path ./project --no-gitignore
```

---

## Search

### grep - Search Files

```bash
# Basic search
llm-support grep "pattern" src/

# Case insensitive
llm-support grep "pattern" src/ -i

# Show line numbers
llm-support grep "pattern" src/ -n

# Both
llm-support grep "TODO" src/ -i -n

# Files only
llm-support grep "pattern" src/ -l
```

### multigrep - Multi-Keyword Search

```bash
# Search multiple keywords
llm-support multigrep --path src/ --keywords "useState,useEffect,useCallback"

# Filter by extension
llm-support multigrep --path src/ --keywords "fn1,fn2" --extensions "ts,tsx"

# Definitions only
llm-support multigrep --path src/ --keywords "handleSubmit" -d

# Case insensitive
llm-support multigrep --path src/ --keywords "TODO,FIXME" -i

# Limit matches per keyword
llm-support multigrep --path src/ --keywords "import" --max-per-keyword 5

# JSON output
llm-support multigrep --path src/ --keywords "fn1,fn2" --json

# Write to files
llm-support multigrep --path src/ --keywords "fn1,fn2" -o ./results/
```

### multiexists - Check File Existence

```bash
# Check multiple files
llm-support multiexists config.json README.md package.json

# Check mixed files and directories
llm-support multiexists src/ tests/ README.md
```

---

## Code Analysis

### detect - Detect Project Type

```bash
# Detect project stack
llm-support detect --path .

# JSON output
llm-support detect --path . --json
```

**Output fields:** STACK, LANGUAGE, PACKAGE_MANAGER, FRAMEWORK, HAS_TESTS

### discover-tests - Find Test Infrastructure

```bash
# Discover test patterns
llm-support discover-tests --path ./project

# JSON output
llm-support discover-tests --path ./project --json
```

**Output fields:** PATTERN, FRAMEWORK, TEST_RUNNER, CONFIG_FILE, SOURCE_DIR, TEST_DIR

### analyze-deps - Extract File Dependencies

```bash
# Analyze markdown for file references
llm-support analyze-deps plan.md

# JSON output
llm-support analyze-deps plan.md --json
```

**Output fields:** FILES_READ, FILES_MODIFY, FILES_CREATE, DIRECTORIES

### partition-work - Group Tasks

```bash
# Partition tasks by file conflicts
llm-support partition-work tasks.md

# JSON output
llm-support partition-work tasks.md --json
```

---

## Data Processing

### json - JSON Operations

```bash
# Pretty-print JSON
llm-support json parse file.json

# Compact output
llm-support json parse file.json --compact

# Custom indent
llm-support json parse file.json --indent 4

# Query with dot notation
llm-support json query data.json ".users[0].name"
llm-support json query data.json ".config.database.host"
llm-support json query data.json ".items[2].price"

# Validate syntax
llm-support json validate file.json

# Merge multiple files
llm-support json merge base.json overrides.json
```

**Query Syntax:**
- `.key` - Access object property
- `[N]` - Access array element (zero-indexed)
- Chain them: `.users[0].address.city`

### toml - TOML Operations

```bash
# Get value by key
llm-support toml get config.toml database.host

# Parse and pretty-print
llm-support toml parse config.toml

# Validate TOML syntax
llm-support toml validate config.toml
```

### markdown - Markdown Processing

```bash
# Generate table of contents
llm-support markdown toc README.md

# Extract headings
llm-support markdown headings doc.md

# Extract links
llm-support markdown links file.md
```

### extract - Pattern Extraction

```bash
# Extract URLs
llm-support extract urls file.txt

# Extract email addresses
llm-support extract emails file.txt

# Extract IP addresses
llm-support extract ips file.txt

# Extract file paths
llm-support extract paths file.txt

# Extract template variables
llm-support extract variables template.txt

# Extract TODOs
llm-support extract todos file.md

# Count only
llm-support extract urls file.txt --count

# Unique values
llm-support extract urls file.txt --unique
```

### transform - Text Transformations

```bash
# Case transforms
llm-support transform upper file.txt
llm-support transform lower file.txt
llm-support transform title file.txt

# Clean whitespace
llm-support transform trim file.txt

# Slugify
llm-support transform slug file.txt
```

### count - Count Items

```bash
# Count checkboxes in markdown
llm-support count --mode checkboxes --path plan.md

# Recursive checkbox count
llm-support count --mode checkboxes --path . -r

# Count lines
llm-support count --mode lines --path file.txt

# Count files
llm-support count --mode files --path src/ --pattern "*.go"
```

**Checkbox output:** TOTAL, CHECKED, UNCHECKED, PERCENT

### encode/decode - Encoding Operations

```bash
# Base64 encode (default)
llm-support encode "hello world"

# Base64 decode
llm-support decode "aGVsbG8gd29ybGQ="

# Hex encode
llm-support encode "hello" -e hex

# URL encode
llm-support encode "hello world" -e url

# Base32 encode
llm-support encode "hello" -e base32
```

### math - Mathematical Expressions

```bash
llm-support math "2 + 3 * 4"              # Result: 14
llm-support math "2**10"                   # Result: 1024
llm-support math "round(22/7, 2)"          # Result: 3.14
llm-support math "max(1, 5, 3)"            # Result: 5
llm-support math "sum(1, 2, 3, 4)"         # Result: 10
llm-support math "abs(-42)"                # Result: 42
```

**Operators:** `+`, `-`, `*`, `/`, `%`, `**`
**Functions:** `abs()`, `round()`, `min()`, `max()`, `sum()`, `pow()`, `sqrt()`

---

## Template Processing

### template - Variable Substitution

```bash
# Basic variable substitution (uses {{var}} syntax by default)
llm-support template file.txt --var name=John --var age=30

# Use brackets syntax [[var]]
llm-support template file.txt --var name=John --syntax brackets

# Load variable from file contents
llm-support template prompt.md --var CONTENT=@document.txt

# Strip whitespace from file values
llm-support template prompt.md --var CONTENT=@document.txt --strip

# Load variables from JSON file
llm-support template file.txt --data vars.json

# Use environment variables
llm-support template file.txt --env

# Combine sources (CLI has highest priority)
llm-support template file.txt --data defaults.json --var name=Override --env

# Write to output file
llm-support template file.txt --var x=y -o result.txt

# Strict mode (error on undefined variables)
llm-support template file.txt --var x=y --strict
```

**Template Syntax (default `--syntax braces`):**
```
Hello {{name}}!                    # Variable
Theme: {{theme|dark}}              # With default value
```

**Alternative Syntax (`--syntax brackets`):**
```
Hello [[name]]!
Theme: [[theme|dark]]
```

---

## LLM Integration

### prompt - Execute LLM Prompts

```bash
# Direct prompt
llm-support prompt --prompt "Explain this code"

# From file
llm-support prompt --file prompt.txt

# With template
llm-support prompt --template prompt.md --var CODE=@file.go

# With retries
llm-support prompt --prompt "Generate code" --retries 3

# With caching
llm-support prompt --prompt "Analyze this" --cache

# With validation
llm-support prompt --prompt "Generate JSON" --min-length 100

# Specify LLM
llm-support prompt --prompt "Hello" --llm gemini

# With system instruction
llm-support prompt --prompt "Review code" --instruction "You are a code reviewer"
```

### foreach - Batch Process Files

```bash
# Process all files in directory
llm-support foreach --path src/ --template review.txt

# Filter by extension
llm-support foreach --path src/ --template prompt.txt --extensions "go"
```

### summarize-dir - Summarize Directory

```bash
# Outline format
llm-support summarize-dir --path src/ --format outline

# Headers only
llm-support summarize-dir --path docs/ --format headers

# Recursive
llm-support summarize-dir --path . -r
```

---

## Validation

### validate - Validate Files

```bash
# Validate JSON
llm-support validate config.json

# Validate YAML
llm-support validate data.yaml

# Validate TOML
llm-support validate settings.toml

# Validate multiple files
llm-support validate config.json settings.yaml
```

### validate-plan - Validate Sprint Plans

```bash
# Validate plan directory
llm-support validate-plan --path ./sprint-01/

# JSON output
llm-support validate-plan --path ./sprint-01/ --json
```

### repo-root - Find Git Repository Root

```bash
# Find repo root from current directory
llm-support repo-root

# Find from specific path
llm-support repo-root --path ./src/components

# Validate .git directory exists
llm-support repo-root --validate

# Both path and validation
llm-support repo-root --path /some/subdir --validate
```

**Output fields:** ROOT, VALID (with --validate)

### highest - Find Highest Numbered Item

```bash
# Find highest plan number
llm-support highest --path .planning/plans

# Find highest sprint
llm-support highest --path .planning/sprints --type dir

# Find highest user story
llm-support highest --path .planning/plans/1.0_feature/user-stories

# Find highest AC for a specific user story (use --prefix)
llm-support highest --path .planning/plans/1.0_feature/acceptance-criteria --prefix "01-"

# Custom pattern
llm-support highest --path ./releases --pattern "^v(\\d+)\\.(\\d+)"

# JSON output
llm-support highest --path .planning/plans --json
```

**Auto-detected patterns by directory:**
- `plans/sprints`: `^(\d+)\.(\d+)[-_]` → extracts "115.0"
- `user-stories`: `^(\d+)[-_]` → extracts "01"
- `acceptance-criteria`: `^(\d+)[-_](\d+)[-_]` → extracts "01-02"
- `tasks`: `^(?:task[-_])?(\d+)[-_]` → extracts "01"
- `technical-debt`: `(?i)^td[-_](\d+)[-_]` → extracts "22"

**Output fields:** HIGHEST, NAME, FULL_PATH, NEXT, COUNT

---

## Session Management

### context - Manage Prompt Variables

Persistent key-value storage for prompt variables across executions.

```bash
# Initialize context in temp directory
llm-support init-temp --name mysession
llm-support context init --dir .planning/.temp/mysession

# Store values
llm-support context set --dir .planning/.temp/mysession MY_VAR "hello"
llm-support context set --dir .planning/.temp/mysession TIMESTAMP "2025-12-29"

# Retrieve values
llm-support context get --dir .planning/.temp/mysession MY_VAR
llm-support context get --dir .planning/.temp/mysession MY_VAR --min
llm-support context get --dir .planning/.temp/mysession MISSING --default "fallback"

# List all values
llm-support context list --dir .planning/.temp/mysession
llm-support context list --dir .planning/.temp/mysession --json

# Source into shell
eval "$(llm-support context dump --dir .planning/.temp/mysession)"

# Clear all values
llm-support context clear --dir .planning/.temp/mysession
```

**Subcommands:** init, set, get, list, dump, clear

### git-changes - Git Working Tree Changes

```bash
# Count all changes
llm-support git-changes

# Filter to specific path
llm-support git-changes --path .planning/

# Staged only
llm-support git-changes --staged-only

# Just the count
llm-support git-changes --min

# JSON output
llm-support git-changes --json
```

**Output fields:** COUNT, FILES

### init-temp - Initialize Temp Directory

```bash
# Create temp directory (cleans existing)
llm-support init-temp --name design-sprint

# Preserve existing files
llm-support init-temp --name cache --preserve

# JSON output
llm-support init-temp --name test --json
```

**Output fields:** TEMP_DIR, STATUS, CLEANED

### plan-type - Extract Plan Type

```bash
# Get plan type
llm-support plan-type --path .planning/plans/1.0_feature/

# Just the type string
llm-support plan-type --min

# JSON output
llm-support plan-type --json
```

**Valid types:** feature, bugfix, test-remediation, tech-debt, infrastructure

**Output fields:** TYPE, LABEL, ICON, SOURCE

### args - Parse Arguments

```bash
# Parse positional arguments
llm-support args file1.txt file2.txt

# Parse file references
llm-support args @.planning/plans/1.0_feature/
```

**Output fields:** POSITIONAL, FLAG_NAME

---

## Flag Reference

| Flag | Commands | Purpose |
|------|----------|---------|
| `--no-gitignore` | listdir, tree, catfiles, grep, stats | Include gitignored files |
| `--sizes` | listdir, tree | Show file sizes |
| `--dates` | listdir | Show modification times |
| `--depth N` | tree | Maximum depth to display |
| `--max-size N` | catfiles | Max total size in MB |
| `-i` | grep, multigrep | Case-insensitive search |
| `-n` | grep | Show line numbers |
| `-l` | grep | Show filenames only |
| `-d` | multigrep | Definitions only |
| `-a ALG` | hash | Hash algorithm (md5/sha1/sha256/sha512) |
| `-e TYPE` | encode/decode | Encoding type (base64/base32/hex/url) |
| `--indent N` | json parse | Indentation spaces |
| `--compact` | json parse | Minify output |
| `--var key=val` | template, prompt | Set variable (literal value) |
| `--var key=@file` | template, prompt | Set variable (file contents) |
| `--data file.json` | template | Load variables from JSON |
| `--env` | template | Use environment variables |
| `-o, --output` | template | Write to file |
| `--strict` | template | Error on undefined variables |
| `--strip` | template, prompt | Strip whitespace from file values |
| `--syntax` | template | Variable syntax: braces or brackets |
| `--json` | detect, multigrep, analyze-deps | Output as JSON |
| `--format` | global | Output format: text, json |
| `-v, --verbose` | global | Enable verbose output |

---

## One-Liners

```bash
# Find all TODOs and FIXMEs
llm-support grep "TODO|FIXME" . -i -n

# Show project structure
llm-support tree --path . --depth 2

# Search for multiple functions
llm-support multigrep --path src/ --keywords "handleSubmit,validateForm" -d

# Get first user from API response
llm-support json query response.json ".users[0]"

# Calculate percentage
llm-support math "round(42/100 * 75, 2)"

# Encode then decode (roundtrip)
llm-support encode "secret" | llm-support decode

# Merge configuration files
llm-support json merge base.json dev.json local.json

# Hash all Go files
llm-support hash **/*.go -a sha256

# Quick stats summary
llm-support stats --path src/

# Count completed tasks
llm-support count --mode checkboxes --path plan.md -r

# Detect project stack
llm-support detect --path . --json

# Validate all config files
llm-support validate config.json settings.yaml

# Generate from template with env vars
llm-support template deploy.tpl --env -o deploy.sh

# Find git repo root
llm-support repo-root --validate

# Find highest plan number and next
llm-support highest --path .planning/plans --json
```

---

**Version:** 1.2.0 | **Go:** 1.22+ | **License:** MIT

See [README.md](../README.md) for full documentation.
