# llm-support Command Reference

Complete documentation for all 40+ llm-support commands.

## Table of Contents

- [File Operations](#file-operations)
  - [listdir](#listdir)
  - [tree](#tree)
  - [catfiles](#catfiles)
  - [hash](#hash)
  - [stats](#stats)
- [Search](#search)
  - [grep](#grep)
  - [multigrep](#multigrep)
  - [multiexists](#multiexists)
- [Code Analysis](#code-analysis)
  - [detect](#detect)
  - [discover-tests](#discover-tests)
  - [analyze-deps](#analyze-deps)
  - [partition-work](#partition-work)
- [Data Processing](#data-processing)
  - [json](#json)
  - [toml](#toml)
  - [markdown](#markdown)
  - [extract](#extract)
  - [transform](#transform)
  - [count](#count)
  - [encode/decode](#encodedecode)
  - [math](#math)
- [Template Processing](#template-processing)
  - [template](#template)
- [LLM Integration](#llm-integration)
  - [prompt](#prompt)
  - [foreach](#foreach)
  - [extract-relevant](#extract-relevant)
  - [summarize-dir](#summarize-dir)
- [Development](#development)
  - [validate](#validate)
  - [validate-plan](#validate-plan)
  - [diff](#diff)
  - [report](#report)
  - [deps](#deps)
  - [git-context](#git-context)
  - [git-changes](#git-changes)
  - [highest](#highest)
  - [init-temp](#init-temp)
  - [plan-type](#plan-type)
  - [repo-root](#repo-root)
- [Session Management](#session-management)
  - [context](#context)
  - [args](#args)

---

## File Operations

### listdir

List directory contents with optional file sizes and dates.

```bash
llm-support listdir [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Directory path (default: ".") |
| `--sizes` | Show file sizes |
| `--dates` | Show modification dates |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support listdir --path src/
llm-support listdir --path src/ --sizes --dates
llm-support listdir --no-gitignore
```

**Output Format:**
```
[type] name [size] [date]
```

---

### tree

Display directory tree structure with optional file sizes.

```bash
llm-support tree [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Directory path (default: ".") |
| `--depth N` | Maximum depth to display (default: 999) |
| `--sizes` | Show file sizes |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support tree --path src/
llm-support tree --path src/ --depth 3
llm-support tree --sizes
```

---

### catfiles

Concatenate multiple files or directory contents with headers.

```bash
llm-support catfiles [paths...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--max-size N` | Maximum total size in MB (default: 10) |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support catfiles src/
llm-support catfiles src/ lib/ --max-size 5
```

Each file is prefixed with a header showing the file path and size.

---

### hash

Generate hash checksums for one or more files.

```bash
llm-support hash [paths...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-a, --algorithm` | Hash algorithm: md5, sha1, sha256, sha512 (default: sha256) |

**Examples:**
```bash
llm-support hash file.txt
llm-support hash file.txt -a md5
llm-support hash *.go -a sha256
```

---

### stats

Display directory statistics including file counts and size breakdown by extension.

```bash
llm-support stats [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Directory path (default: ".") |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support stats --path ./project
llm-support stats --no-gitignore
```

---

## Search

### grep

Search for a pattern in files using regular expressions.

```bash
llm-support grep [pattern] [paths...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-i, --ignore-case` | Case insensitive search |
| `-n, --line-number` | Show line numbers |
| `-l, --files-with-matches` | Only show file names |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support grep "TODO" src/
llm-support grep "TODO\|FIXME" . -i -n
llm-support grep "func.*Error" internal/ -n
llm-support grep "pattern" src/ -l
```

---

### multigrep

Search for multiple keywords in parallel with intelligent output management.

```bash
llm-support multigrep [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Path to search in (required) |
| `--keywords` | Comma-separated keywords to search (required) |
| `--extensions` | Filter by file extensions (e.g., 'ts,tsx,js') |
| `--max-per-keyword N` | Max matches per keyword (default: 10) |
| `-i, --ignore-case` | Case-insensitive search |
| `-d, --definitions` | Show only definition matches |
| `-o, --output-dir` | Write results to directory (one file per keyword) |
| `--json` | Output as JSON |
| `--no-exclude` | Don't exclude common directories |

**Examples:**
```bash
llm-support multigrep --path src/ --keywords "useState,useEffect"
llm-support multigrep --path src/ --keywords "fn1,fn2" --extensions "ts,tsx"
llm-support multigrep --path src/ --keywords "handleSubmit" -d
llm-support multigrep --path . --keywords "TODO,FIXME" -i --max-per-keyword 20
```

Prioritizes definition matches (function, class, const declarations) over usage matches.

---

### multiexists

Check if multiple files or directories exist.

```bash
llm-support multiexists [paths...]
```

**Examples:**
```bash
llm-support multiexists config.json README.md package.json
llm-support multiexists src/ tests/ docs/
```

---

## Code Analysis

### detect

Detect project type, language, package manager, and framework.

```bash
llm-support detect [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Project path (default: ".") |
| `--json` | Output as JSON |

**Output Fields:**
| Field | Values |
|-------|--------|
| STACK | node, python, go, rust, java, ruby, php, dotnet, unknown |
| LANGUAGE | typescript, javascript, python, go, rust, java, ruby, php, csharp, unknown |
| PACKAGE_MANAGER | npm, yarn, pnpm, pip, poetry, go, cargo, maven, bundler, composer |
| FRAMEWORK | nextjs, remix, express, fastapi, django, flask, gin, actix, spring, rails |
| HAS_TESTS | true, false |

**Examples:**
```bash
llm-support detect
llm-support detect --path ./project --json
```

---

### discover-tests

Discover test patterns, runners, and infrastructure in a project.

```bash
llm-support discover-tests [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Project path (default: ".") |
| `--json` | Output as JSON |

**Output Fields:**
| Field | Description |
|-------|-------------|
| PATTERN | SEPARATED, COLOCATED, UNKNOWN |
| FRAMEWORK | wasp, nextjs, nuxt, angular, vue, remix |
| TEST_RUNNER | vitest, jest, mocha, pytest |
| CONFIG_FILE | Path to test config file |
| SOURCE_DIR | Source directory |
| TEST_DIR | Test directory |
| E2E_DIR | E2E test directory |
| UNIT_TEST_COUNT | Number of unit test files |
| E2E_TEST_COUNT | Number of e2e test files |

**Examples:**
```bash
llm-support discover-tests --path ./project
llm-support discover-tests --json
```

---

### analyze-deps

Analyze file dependencies from a user story or task markdown file.

```bash
llm-support analyze-deps <file> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

**Output Fields:**
| Field | Description |
|-------|-------------|
| FILES_READ | Files that need to be read |
| FILES_MODIFY | Files that need to be modified |
| FILES_CREATE | Files that need to be created |
| DIRECTORIES | Directories referenced |
| TOTAL_FILES | Total file count |
| CONFIDENCE | high, medium, low |

**Examples:**
```bash
llm-support analyze-deps plan.md
llm-support analyze-deps user-stories/US-001.md --json
```

---

### partition-work

Partition work items (stories/tasks) into parallel groups using graph coloring.

```bash
llm-support partition-work [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--stories` | Directory containing story markdown files |
| `--tasks` | Directory containing task markdown files |
| `--verbose` | Show conflict details |
| `--json` | Output as JSON |

Items that share file dependencies cannot run in parallel and are placed in different groups.

**Examples:**
```bash
llm-support partition-work --stories ./user-stories/
llm-support partition-work --tasks ./tasks/ --verbose
```

---

## Data Processing

### json

Commands for parsing, querying, validating, and merging JSON files.

#### json parse
```bash
llm-support json parse <file> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--compact` | Minify output |
| `--indent N` | Indentation spaces (default: 2) |

#### json query
```bash
llm-support json query <file> <path>
```

Query syntax:
- `.key` - Access object property
- `[N]` - Access array element (zero-indexed)
- Chain: `.users[0].address.city`

#### json validate
```bash
llm-support json validate <file>
```

#### json merge
```bash
llm-support json merge <files...>
```

**Examples:**
```bash
llm-support json parse file.json
llm-support json parse file.json --compact
llm-support json query data.json ".users[0].name"
llm-support json validate config.json
llm-support json merge base.json overrides.json
```

---

### toml

Commands for parsing, querying, and validating TOML files.

#### toml get
```bash
llm-support toml get <file> <key>
```

#### toml parse
```bash
llm-support toml parse <file>
```

#### toml validate
```bash
llm-support toml validate <file>
```

**Examples:**
```bash
llm-support toml get config.toml database.host
llm-support toml parse config.toml
llm-support toml validate settings.toml
```

---

### markdown

Commands for parsing and extracting content from Markdown files.

#### markdown headers
```bash
llm-support markdown headers <file>
```

#### markdown frontmatter
```bash
llm-support markdown frontmatter <file>
```

#### markdown section
```bash
llm-support markdown section <file> --title "Section Title"
```

#### markdown codeblocks
```bash
llm-support markdown codeblocks <file>
```

#### markdown tasks
```bash
llm-support markdown tasks <file>
```

**Examples:**
```bash
llm-support markdown headers README.md
llm-support markdown frontmatter post.md
llm-support markdown section doc.md --title "Installation"
llm-support markdown codeblocks tutorial.md
```

---

### extract

Extract various patterns from text files.

```bash
llm-support extract <type> <file> [flags]
```

**Types:**
| Type | Description |
|------|-------------|
| `urls` | Extract URLs (http/https) |
| `paths` | Extract file paths |
| `variables` | Extract template variables {{var}} |
| `todos` | Extract TODO checkboxes |
| `emails` | Extract email addresses |
| `ips` | Extract IP addresses |

**Flags:**
| Flag | Description |
|------|-------------|
| `--count` | Show count only |
| `--unique` | Remove duplicates |

**Examples:**
```bash
llm-support extract urls file.txt
llm-support extract emails contacts.txt --unique
llm-support extract todos plan.md --count
```

---

### transform

Text and data transformations.

```bash
llm-support transform <operation> <file>
```

**Operations:**
| Operation | Description |
|-----------|-------------|
| `upper` | Convert to uppercase |
| `lower` | Convert to lowercase |
| `title` | Convert to title case |
| `trim` | Strip whitespace |
| `slug` | Convert to URL slug |

**Examples:**
```bash
llm-support transform upper file.txt
llm-support transform slug title.txt
```

---

### count

Count checkboxes, lines, or files in a path.

```bash
llm-support count [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--mode` | Count mode: checkboxes, lines, files (required) |
| `--path` | Path to count in (required) |
| `-r, --recursive` | Recursive search |
| `--pattern` | Glob pattern (for files mode) |
| `--style` | Checkbox style: all, list, heading (default: all) |

**Checkbox Output:**
| Field | Description |
|-------|-------------|
| TOTAL | Total checkboxes |
| CHECKED | Completed checkboxes [x] |
| UNCHECKED | Incomplete checkboxes [ ] |
| PERCENT | Completion percentage |

**Examples:**
```bash
llm-support count --mode checkboxes --path plan.md
llm-support count --mode lines --path file.txt
llm-support count --mode files --path src/ --pattern "*.go" -r
```

---

### encode/decode

Encode or decode text using various encodings.

```bash
llm-support encode [text...] [flags]
llm-support decode [text...] [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-e, --encoding` | Encoding type: base64, base32, hex, url (default: base64) |

**Examples:**
```bash
llm-support encode "hello world"
llm-support encode "hello" -e hex
llm-support decode "aGVsbG8gd29ybGQ="
llm-support decode "68656c6c6f" -e hex
```

---

### math

Evaluate mathematical expressions safely.

```bash
llm-support math "<expression>"
```

**Operators:** `+`, `-`, `*`, `/`, `%`, `**`

**Functions:** `abs()`, `round()`, `min()`, `max()`, `sum()`, `pow()`, `sqrt()`

**Examples:**
```bash
llm-support math "2 + 3 * 4"        # 14
llm-support math "2**10"            # 1024
llm-support math "round(22/7, 2)"   # 3.14
llm-support math "max(1, 5, 3)"     # 5
llm-support math "sqrt(16)"         # 4
```

---

## Template Processing

### template

Perform variable substitution in template files.

```bash
llm-support template <file> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--var KEY=VALUE` | Set variable (literal value) |
| `--var KEY=@file` | Set variable (file contents) |
| `--data file.json` | Load variables from JSON file |
| `--env` | Include environment variables |
| `-o, --output` | Output file (default: stdout) |
| `--strict` | Error on undefined variables |
| `--strip` | Strip whitespace from file values |
| `--syntax` | Variable syntax: braces ({{var}}) or brackets ([[var]]) (default: braces) |

**Template Syntax:**
```
Hello {{name}}!                    # Variable
Theme: {{theme|dark}}              # With default value
```

**Examples:**
```bash
llm-support template file.txt --var name=John --var age=30
llm-support template prompt.md --var CONTENT=@document.txt --strip
llm-support template config.tpl --data vars.json --env
llm-support template file.txt --var x=y -o result.txt
llm-support template file.txt --syntax brackets --var name=John
```

---

## LLM Integration

### prompt

Execute an LLM prompt with template substitution, retry logic, and validation.

```bash
llm-support prompt [flags]
```

**Input Sources (mutually exclusive):**
| Flag | Description |
|------|-------------|
| `--prompt` | Direct prompt text |
| `--file` | Read prompt from file |
| `--template` | Template file with [[var]] placeholders |

**Flags:**
| Flag | Description |
|------|-------------|
| `--llm` | LLM binary to use (default: from config or 'gemini') |
| `--instruction` | System instruction for the LLM |
| `--var KEY=VALUE` | Template variable |
| `--retries N` | Number of retries on failure |
| `--retry-delay N` | Initial retry delay in seconds (default: 2) |
| `--cache` | Enable response caching |
| `--cache-ttl N` | Cache TTL in seconds (default: 3600) |
| `--refresh` | Force refresh cached response |
| `--min-length N` | Minimum response length |
| `--must-contain` | Required text in response |
| `--no-error-check` | Skip error pattern checking |
| `--timeout N` | Timeout in seconds (default: 120) |
| `--output` | Output file |
| `--strip` | Strip whitespace from file variable values |

**Examples:**
```bash
llm-support prompt --prompt "Explain this code"
llm-support prompt --file prompt.txt --llm claude
llm-support prompt --template prompt.md --var CODE=@file.go
llm-support prompt --prompt "Generate JSON" --retries 3 --min-length 100
llm-support prompt --prompt "Analyze" --cache --instruction "You are a code reviewer"
```

---

### foreach

Process multiple files through an LLM using a template.

```bash
llm-support foreach [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--template` | Template file with [[var]] placeholders (required) |
| `--files` | Files to process (comma-separated or repeated) |
| `--glob` | Glob pattern to match files |
| `--llm` | LLM binary to use |
| `--output-dir` | Output directory for processed files |
| `--output-pattern` | Output filename pattern (e.g., '{{name}}-processed.md') |
| `--parallel N` | Number of parallel processes (default: 1) |
| `--skip-existing` | Skip files where output already exists |
| `--var KEY=VALUE` | Template variable |
| `--timeout N` | Timeout per file in seconds (default: 120) |
| `--json` | Output results as JSON |

**Template Variables:**
| Variable | Description |
|----------|-------------|
| `[[CONTENT]]` | Content of the current file |
| `[[FILENAME]]` | Base name of the current file |
| `[[FILEPATH]]` | Full path of the current file |
| `[[EXTENSION]]` | File extension |
| `[[DIRNAME]]` | Directory name |
| `[[INDEX]]` | 1-based index of current file |
| `[[TOTAL]]` | Total number of files |

**Examples:**
```bash
llm-support foreach --files "*.md" --template prompt.md --output-dir ./out
llm-support foreach --glob "src/**/*.go" --template analyze.md --llm claude --parallel 4
llm-support foreach --files file1.txt,file2.txt --template t.md --var LANG=Go --skip-existing
```

---

### extract-relevant

Extract relevant content from files or directories using an LLM API.

```bash
llm-support extract-relevant [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | File or directory path (default: ".") |
| `--context` | Context describing what content to extract (required) |
| `--concurrency N` | Number of concurrent API calls (default: 2) |
| `-o, --output` | Output file (default: stdout) |
| `--timeout N` | API call timeout in seconds (default: 60) |
| `--json` | Output as JSON |

**API Configuration:**
- Environment variable: `OPENAI_API_KEY`
- Or file: `.planning/.config/openai_api_key`
- Optional: `OPENAI_BASE_URL`, `OPENAI_MODEL`

**Examples:**
```bash
llm-support extract-relevant --path ./src --context "API endpoint definitions"
llm-support extract-relevant --path ./docs --context "Configuration options" --concurrency 4
llm-support extract-relevant --path ./file.md --context "Code examples" -o output.md
```

---

### summarize-dir

Generate a summary of directory contents for LLM context.

```bash
llm-support summarize-dir [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Directory path (default: ".") |
| `--format` | Output format: tree, outline, full (default: tree) |
| `--glob` | File glob pattern |
| `--lines N` | Max lines per file in outline mode (default: 10) |
| `--max-tokens N` | Approximate max tokens (default: 4000) |
| `-r, --recursive` | Recursive scan (default: true) |
| `--no-gitignore` | Include gitignored files |

**Examples:**
```bash
llm-support summarize-dir --path src/
llm-support summarize-dir --path docs/ --format outline
llm-support summarize-dir --format full --max-tokens 8000
```

---

## Development

### validate

Validate files of various formats.

```bash
llm-support validate <files...>
```

**Supported Formats:**
| Extension | Validation |
|-----------|------------|
| `.json` | JSON syntax |
| `.toml` | TOML syntax |
| `.yml/.yaml` | YAML structure |
| `.csv` | CSV structure |
| `.md` | Non-empty check |

**Examples:**
```bash
llm-support validate config.json
llm-support validate config.json settings.yaml data.toml
```

---

### validate-plan

Validate that a plan directory has the required structure.

```bash
llm-support validate-plan [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Path to plan directory (default: current directory) |
| `--json` | Output as JSON |

**Required Structure:**
- `plan.md` (file)
- `user-stories/` (directory)
- `acceptance-criteria/` (directory)

**Optional:**
- `original-request.md`
- `sprint-design.md`
- `metadata.md`
- `README.md`

**Examples:**
```bash
llm-support validate-plan --path .planning/plans/my-plan
llm-support validate-plan --path ./sprint-01/ --json
```

---

### diff

Compare two files and show differences.

```bash
llm-support diff <file1> <file2>
```

**Examples:**
```bash
llm-support diff file1.txt file2.txt
llm-support diff old/config.json new/config.json
```

---

### report

Generate a formatted markdown status report.

```bash
llm-support report [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--title` | Report title (required) |
| `--status` | Report status: success, partial, failed (required) |
| `--stat KEY=VALUE` | Statistics (can be repeated) |
| `-o, --output` | Output file (default: stdout) |

**Examples:**
```bash
llm-support report --title "Build Report" --status success
llm-support report --title "Test Results" --stat tests=50 --stat passed=48 --stat failed=2 --status partial
llm-support report --title "Deploy" --status success -o report.md
```

---

### deps

Parse and list dependencies from package manifest files.

```bash
llm-support deps <manifest> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--type` | Dependency type: all, prod, dev (default: all) |
| `--json` | Output as JSON |

**Supported Manifests:**
- `package.json` (Node.js)
- `go.mod` (Go)
- `requirements.txt` (Python)
- `Cargo.toml` (Rust)
- `Gemfile` (Ruby)
- `pom.xml` (Maven)
- `pyproject.toml` (Python)

**Examples:**
```bash
llm-support deps package.json
llm-support deps go.mod --type prod
llm-support deps requirements.txt --json
```

---

### git-context

Gather comprehensive git repository information.

```bash
llm-support git-context [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Path to git repository (default: current directory) |
| `--include-diff` | Include diff of uncommitted changes |
| `--max-commits N` | Maximum number of commits to include (default: 10) |
| `--since YYYY-MM-DD` | Only include commits since date |
| `--json` | Output as JSON |

**Examples:**
```bash
llm-support git-context
llm-support git-context --path /path/to/repo
llm-support git-context --include-diff
llm-support git-context --since 2025-12-01 --max-commits 20
llm-support git-context --json
```

---

### git-changes

Count and list git working tree changes with optional path filtering.

```bash
llm-support git-changes [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Filter to files matching this path prefix |
| `--include-untracked` | Include untracked files (default: true) |
| `--staged-only` | Only show staged changes |
| `--json` | Output as JSON |
| `--min` | Minimal output (count only) |

**Output Format:**
```
COUNT: 5
FILES:
  M  src/main.go
  A  src/new.go
  ?? untracked.txt
```

**Examples:**
```bash
# Count all changes
llm-support git-changes

# Filter to planning directory
llm-support git-changes --path .planning/

# Only staged changes
llm-support git-changes --staged-only

# Exclude untracked files
llm-support git-changes --include-untracked=false

# Just the count
llm-support git-changes --min

# JSON output
llm-support git-changes --json
```

---

### repo-root

Find and output git repository root path.

```bash
llm-support repo-root [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Starting path to search from (default: current directory) |
| `--validate` | Verify .git directory exists at root |

**Output Format:**
```
ROOT: /absolute/path/to/repo
VALID: TRUE|FALSE  (only with --validate)
```

**Examples:**
```bash
llm-support repo-root
llm-support repo-root --path ./src/components
llm-support repo-root --validate
llm-support repo-root --path /path/to/subdir --validate
```

---

### highest

Find the highest numbered directory or file in a path. Useful for determining the next plan number, sprint number, user story number, etc.

```bash
llm-support highest [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Directory to search in (default: ".") |
| `--pattern` | Custom regex pattern (auto-detected if not provided) |
| `--type` | Type to search: dir, file, both (default: both) |
| `--prefix` | Filter to items starting with this prefix |
| `--json` | Output as JSON |

**Auto-detected Patterns by Directory Name:**
| Directory | Pattern | Example Match |
|-----------|---------|---------------|
| plans, sprints | `^(\d+)\.(\d+)[-_]` | `115.0_feature` |
| user-stories | `^(\d+)[-_]` | `01-story-name` |
| acceptance-criteria | `^(\d+)[-_](\d+)[-_]` | `01-02-criteria` |
| tasks | `^(?:task[-_])?(\d+)[-_]` | `task-01-name` |
| technical-debt | `(?i)^td[-_](\d+)[-_]` | `td-22-item` |

**Output Format:**
```
HIGHEST: 115.0
NAME: 115.0_feature_name
FULL_PATH: /path/to/115.0_feature_name
NEXT: 116.0
COUNT: 8
```

**Examples:**
```bash
# Find highest plan number
llm-support highest --path .planning/plans --type dir

# Find highest user story
llm-support highest --path .planning/plans/X/user-stories --type file

# Find highest AC for user story 01
llm-support highest --path .planning/plans/X/acceptance-criteria --prefix "01-"

# JSON output
llm-support highest --path .planning/sprints/active --type dir --json

# Custom pattern
llm-support highest --path ./releases --pattern "^v(\d+)\.(\d+)"
```

---

### init-temp

Initialize and manage temp directories with consistent patterns.

```bash
llm-support init-temp [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--name` | Name for temp directory (required) |
| `--clean` | Remove existing files (default: true) |
| `--preserve` | Keep existing files if directory exists |
| `--json` | Output as JSON |
| `--min` | Minimal output |

**Output Format:**
```
TEMP_DIR: .planning/.temp/mycontext
STATUS: CREATED
CLEANED: 3 files removed
```

**Examples:**
```bash
# Create temp directory (cleans existing)
llm-support init-temp --name design-sprint

# Preserve existing files
llm-support init-temp --name cache --preserve

# JSON output
llm-support init-temp --name test --json
```

---

### plan-type

Extract plan type from metadata.md or plan.md with intelligent fallback.

```bash
llm-support plan-type [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--path` | Plan directory path (default: ".") |
| `--json` | Output as JSON |
| `--min` | Minimal output (type only) |

**Valid Plan Types:**
- `feature` - New functionality
- `bugfix` - Bug fixes
- `test-remediation` - Test improvements
- `tech-debt` - Technical debt cleanup
- `infrastructure` - Infrastructure changes

**Output Format:**
```
TYPE: feature
LABEL: Feature
ICON: ✨
SOURCE: metadata.md
```

**Examples:**
```bash
# Get plan type from current directory
llm-support plan-type

# Specify plan path
llm-support plan-type --path .planning/plans/8.1_feature/

# Just the type string
llm-support plan-type --min

# JSON output
llm-support plan-type --json
```

---

## Session Management

Commands for managing persistent state across prompt executions.

### context

Manage persistent key-value storage for prompt variables. Solves the "forgotten timestamp" problem where LLMs lose precise values during long-running prompts.

```bash
llm-support context <subcommand> [flags]
```

**Subcommands:**

#### context init

Initialize a context.env file in a directory.

```bash
llm-support context init --dir <directory>
```

**Output:**
```
CONTEXT_FILE: /tmp/mycontext/context.env
STATUS: CREATED
```

#### context set

Store a key-value pair. Keys are automatically uppercased.

```bash
llm-support context set --dir <directory> KEY VALUE
```

**Examples:**
```bash
llm-support context set --dir /tmp/ctx MY_VAR "hello world"
llm-support context set --dir /tmp/ctx TIMESTAMP "2025-12-29T10:00:00Z"
llm-support context set --dir /tmp/ctx MESSAGE "It's working!"
```

#### context get

Retrieve a value by key.

```bash
llm-support context get --dir <directory> KEY [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--default` | Default value if key not found |
| `--json` | Output as JSON |
| `--min` | Output just the value |

**Examples:**
```bash
llm-support context get --dir /tmp/ctx MY_VAR
llm-support context get --dir /tmp/ctx MISSING --default "fallback"
llm-support context get --dir /tmp/ctx MY_VAR --min
llm-support context get --dir /tmp/ctx MY_VAR --json
```

#### context list

List all stored key-value pairs.

```bash
llm-support context list --dir <directory> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output as JSON object |
| `--min` | Output values only |

**Examples:**
```bash
llm-support context list --dir /tmp/ctx
llm-support context list --dir /tmp/ctx --json
```

#### context dump

Output in shell-sourceable format for use with `eval`.

```bash
llm-support context dump --dir <directory>
```

**Example:**
```bash
# Source context variables into shell
eval "$(llm-support context dump --dir /tmp/ctx)"
echo $MY_VAR
```

#### context clear

Remove all values from the context file (preserves header).

```bash
llm-support context clear --dir <directory>
```

**Common Workflow:**
```bash
# 1. Create temp directory and initialize context
TEMP=$(llm-support init-temp --name mysession | grep TEMP_DIR | cut -d' ' -f2)
llm-support context init --dir "$TEMP"

# 2. Store values during prompt execution
llm-support context set --dir "$TEMP" START_TIME "$(date -Iseconds)"
llm-support context set --dir "$TEMP" BRANCH_NAME "feature/new-thing"

# 3. Retrieve values later
llm-support context get --dir "$TEMP" START_TIME --min

# 4. Source all variables
eval "$(llm-support context dump --dir "$TEMP")"
```

---

### args

Parse command-line arguments into structured format. Useful for skill/prompt argument handling.

```bash
llm-support args [arguments...]
```

**Output Format:**
```
POSITIONAL: value1 value2
FLAG_NAME: flag_value
```

**Examples:**
```bash
# Parse positional arguments
llm-support args file1.txt file2.txt
# Output: POSITIONAL: file1.txt file2.txt

# Parse file references
llm-support args @.planning/plans/1.0_feature/
# Output: POSITIONAL: @.planning/plans/1.0_feature/
```

---

## Global Flags

These flags are available for all commands:

| Flag | Description |
|------|-------------|
| `--format` | Output format: text, json (default: text) |
| `-v, --verbose` | Enable verbose output |
| `--no-gitignore` | Disable .gitignore filtering |
| `-h, --help` | Help for command |
| `--version` | Version information |

---

## See Also

- [README.md](../README.md) - Main documentation
- [quick-reference.md](quick-reference.md) - Command cheat sheet
- [MCP_SETUP.md](MCP_SETUP.md) - Claude Desktop integration
