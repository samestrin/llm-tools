# Correct YAML Structure - llm-support Configuration Schema

**Date**: 2025-12-30
**Scope**: llm-tools repository (llm-support MCP)
**Consumer**: claude-prompts repository

---

## Overview

Define and implement the canonical YAML configuration structure for the planning system. The `llm_support_yaml_*` tools will read/write this structure. This document specifies:

1. The expected schema
2. Default values behavior
3. Error handling for missing files/keys
4. Template generation

---

## Canonical Schema

**File**: `.planning/.config/config.yaml`

```yaml
# Planning System Configuration
# ══════════════════════════════════════════════════════════════════
# This file consolidates all prompt configuration settings.
# Prompts read values via llm_support_yaml_multiget with defaults.
# ══════════════════════════════════════════════════════════════════

# ──────────────────────────────────────────────────────────────────
# Tool Binaries
# Paths to CLI tools. Used as fallback when MCP unavailable,
# or when LLM invokes binary directly instead of MCP.
# ──────────────────────────────────────────────────────────────────
binaries:
  llm: /usr/local/bin/llm                    # Helper LLM CLI
  support: /usr/local/bin/llm-support        # This tool (llm-support)
  clarification: /usr/local/bin/llm-clarification
  filesystem: /usr/local/bin/fast-filesystem
  semantic: /usr/local/bin/llm-semantic

# ──────────────────────────────────────────────────────────────────
# Helper LLM Configuration
# Settings for compression, summarization, and helper tasks.
# The helper LLM handles large content that would overflow context.
# ──────────────────────────────────────────────────────────────────
helper:
  provider: gemini                           # gemini | claude | openai | ollama
  model: ""                                  # Model override (empty = provider default)
  max_lines: 1500                            # Target line count for compression

# ──────────────────────────────────────────────────────────────────
# Project Settings
# Auto-detected by /init-specs or llm_support_detect.
# User can override detected values here.
# ──────────────────────────────────────────────────────────────────
project:
  source_directory: src                      # Main source code location
  type: ""                                   # typescript | python | go | rust | java
  framework: ""                              # next | react | vue | fastapi | gin | etc.
  test_framework: ""                         # vitest | jest | pytest | go test

# Testing Commands
# Commands for running tests and coverage.
testing:
  cmd: ""                                    # npm test | pytest | go test ./...
  coverage_cmd: ""                           # npm run coverage | pytest --cov
```

---

## Key Paths Reference

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `binaries.llm` | string | `/usr/local/bin/llm` | Helper LLM binary path |
| `binaries.support` | string | `/usr/local/bin/llm-support` | llm-support binary |
| `binaries.clarification` | string | `/usr/local/bin/llm-clarification` | llm-clarification binary |
| `binaries.filesystem` | string | `/usr/local/bin/fast-filesystem` | fast-filesystem binary |
| `binaries.semantic` | string | `/usr/local/bin/llm-semantic` | llm-semantic binary |
| `helper.provider` | string | `gemini` | LLM provider for helper tasks |
| `helper.model` | string | `""` | Model override |
| `helper.max_lines` | int | `1500` | Compression target |
| `project.source_directory` | string | `src` | Source code root |
| `project.type` | string | `""` | Detected/configured language |
| `project.framework` | string | `""` | Detected/configured framework |
| `project.test_framework` | string | `""` | Test framework in use |
| `testing.cmd` | string | `""` | Command to run tests |
| `testing.coverage_cmd` | string | `""` | Command to run coverage |

---

## Tool Behavior Requirements

### `llm_support_yaml_get`

```
llm_support_yaml_get --file config.yaml --key "helper.provider" --default "gemini"
```

- If file missing: return default (no error)
- If key missing: return default (no error)
- If key exists but empty string: return empty string (NOT default)
- If key exists with value: return value

### `llm_support_yaml_multiget`

```
llm_support_yaml_multiget \
  --file config.yaml \
  --keys '["binaries.llm", "helper.provider", "helper.max_lines"]' \
  --defaults '{"helper.provider": "gemini", "helper.max_lines": "1500"}'
```

- Returns object with all requested keys
- Missing keys filled from defaults
- File missing: all keys use defaults (no error)
- `--min` output: values only, newline-separated (same order as keys)

### `llm_support_yaml_set`

```
llm_support_yaml_set \
  --file config.yaml \
  --key "helper.provider" \
  --value "claude" \
  --create
```

- `--create`: Create file with proper structure if missing
- Creates intermediate keys: `helper.provider` creates `helper:` section
- Preserves comments in existing file
- Validates key format (must be valid YAML path)

### `llm_support_yaml_multiset`

```
llm_support_yaml_multiset \
  --file config.yaml \
  --pairs '{"helper.provider": "claude", "helper.max_lines": "2000"}' \
  --create
```

- Atomic: validates all keys before writing any
- Same create/preserve behavior as `yaml_set`

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| File doesn't exist | Return defaults, no error |
| File exists but empty | Return defaults, no error |
| File is invalid YAML | Return error with line number |
| Key path is malformed | Return error describing valid format |
| Value type mismatch | Accept as string (YAML is stringly typed) |

---

## Template Generation

Consider adding a command to generate template:

```
llm_support_yaml_template --type planning-config --output .planning/.config/config.yaml
```

Or document that `yaml_multiset --create` with full defaults creates valid initial file.

---

## Migration Support

For repos with legacy `.planning/.config/<file>` structure:

1. Detect legacy files exist
2. Read values from each
3. Map to new YAML keys:

| Legacy File | YAML Key |
|-------------|----------|
| `helper_script` | `binaries.llm` |
| `helper_llm` | `helper.provider` |
| `helper_llm_cmd` | (deprecated, remove) |
| `max_lines` | `helper.max_lines` |
| `source_directory` | `project.source_directory` |
| `project_type` | `project.type` |
| `framework` | `project.framework` |
| `clarification_script` | `binaries.clarification` |

4. Write consolidated `config.yaml`
5. Optionally remove legacy files

---

## Testing Checklist

- [ ] `yaml_get` returns default when file missing
- [ ] `yaml_get` returns default when key missing
- [ ] `yaml_get` returns empty string when key is `""`
- [ ] `yaml_multiget` handles partial defaults
- [ ] `yaml_set --create` creates proper nested structure
- [ ] `yaml_multiset` is atomic (all-or-nothing)
- [ ] Comments preserved on update
- [ ] Array index access works: `binaries[0]` (if needed)
- [ ] Deeply nested paths work: `a.b.c.d.e`

---

## Integration with claude-prompts

The claude-prompts repo will update all `.claude/commands/*.md` to use:

```markdown
Use `llm_support_yaml_multiget`:
- file: `${REPO_ROOT}/.planning/.config/config.yaml`
- keys: `["binaries.llm", "helper.provider", "helper.max_lines"]`
- defaults: `{"helper.provider": "gemini", "helper.max_lines": "1500"}`
```

This replaces the current pattern of multiple `cat` commands with fallbacks.

---

## Files NOT in config.yaml

These remain separate:

| File | Reason |
|------|--------|
| `.active_sprint` | Sentinel file - existence is the signal |
| `clarification-tracking.yaml` | Legacy; SQLite is primary storage |

---

## Notes

- All values are strings (YAML is stringly typed for config)
- Empty string `""` means "explicitly unset" (different from missing)
- Prompts should always provide defaults for required values
- `binaries.*` paths enable fallback when MCP unavailable
