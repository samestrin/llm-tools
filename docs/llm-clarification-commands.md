# llm-clarification Command Reference

Complete documentation for all llm-clarification commands.

## Overview

The Clarification Learning System tracks questions and answers gathered during LLM-assisted development. It helps:

- **Avoid repeating questions** - Match new questions against existing clarifications
- **Identify patterns** - Cluster similar questions to find common themes
- **Promote knowledge** - Move frequently-asked clarifications to permanent docs
- **Detect conflicts** - Find contradictory answers across sprints

## Table of Contents

- [Storage Backends](#storage-backends)
- [Management Commands](#management-commands)
  - [init-tracking](#init-tracking)
  - [add-clarification](#add-clarification)
  - [list-entries](#list-entries)
  - [promote-clarification](#promote-clarification)
  - [delete-clarification](#delete-clarification)
- [Storage Commands](#storage-commands)
  - [export-memory](#export-memory)
  - [import-memory](#import-memory)
  - [optimize-memory](#optimize-memory)
  - [reconcile-memory](#reconcile-memory)
- [Analysis Commands (Require API)](#analysis-commands-require-api)
  - [match-clarification](#match-clarification)
  - [cluster-clarifications](#cluster-clarifications)
  - [detect-conflicts](#detect-conflicts)
  - [validate-clarifications](#validate-clarifications)
- [Optimization Commands (Require API)](#optimization-commands-require-api)
  - [normalize-clarification](#normalize-clarification)
  - [suggest-consolidation](#suggest-consolidation)
  - [identify-candidates](#identify-candidates)

---

## API Configuration

Commands marked **(Require API)** need an OpenAI-compatible API.

**Option A: Environment Variables**
```bash
export OPENAI_API_KEY=your-api-key
export OPENAI_BASE_URL=https://openrouter.ai/api/v1  # optional
export OPENAI_MODEL=gpt-4o-mini                       # optional
```

**Option B: Config Files**
```bash
mkdir -p .planning/.config
echo 'your-api-key' > .planning/.config/openai_api_key
echo 'https://openrouter.ai/api/v1' > .planning/.config/openai_base_url
echo 'gpt-4o-mini' > .planning/.config/openai_model
```

---

## Storage Backends

The clarification system supports two storage backends:

### YAML Storage (Default)
- Human-readable and editable
- Best for small to medium datasets (<1000 entries)
- Files: `.yaml`, `.yml`

### SQLite Storage
- High performance for large datasets
- Full-text search capability
- Best for 1000+ entries
- Files: `.db`, `.sqlite`, `.sqlite3`

**Using SQLite:**
```bash
# Initialize with SQLite
llm-clarification init-tracking -o clarifications.db

# Use --db flag to override storage path globally
llm-clarification --db clarifications.db list-entries

# Or set environment variable
export CLARIFY_DB_PATH=clarifications.db
```

**Storage Selection:**
- File extension determines backend automatically
- `--db` flag overrides per-command `--file` flags
- `CLARIFY_DB_PATH` environment variable provides default

---

## Tracking File Format

The tracking file is a YAML document with the following structure:

```yaml
version: "1.0"
entries:
  - id: "CLR-001"
    question: "Should we use Tailwind or CSS modules?"
    answer: "Use Tailwind for this project"
    sprint_id: "sprint-01"
    status: "pending"  # pending, promoted, dismissed
    occurrences: 2
    context_tags:
      - "styling"
      - "frontend"
    created_at: "2025-12-24T10:00:00Z"
    updated_at: "2025-12-24T10:00:00Z"
```

---

## Management Commands

### init-tracking

Initialize a new clarification tracking file with the proper schema.

```bash
llm-clarification init-tracking [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-o, --output` | Output file path (required) |
| `--force` | Overwrite if file exists |

**Examples:**
```bash
llm-clarification init-tracking -o clarifications.yaml
llm-clarification init-tracking -o .planning/clarifications.yaml --force
```

**Output:**
Creates a new YAML file with the tracking schema:
```yaml
version: "1.0"
entries: []
```

---

### add-clarification

Add a new clarification entry or update an existing one.

```bash
llm-clarification add-clarification [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `-q, --question` | Question text |
| `-a, --answer` | Answer text |
| `--id` | Entry ID (for updates, auto-generated for new) |
| `-s, --sprint` | Sprint name/ID |
| `-t, --tag` | Context tags (can be repeated) |
| `--check-match` | Check for similar existing questions before adding |

**Examples:**
```bash
# Add new clarification
llm-clarification add-clarification \
  -f tracking.yaml \
  -q "Should we use Tailwind or CSS modules?" \
  -a "Use Tailwind for this project"

# Add with sprint and tags
llm-clarification add-clarification \
  -f tracking.yaml \
  -q "Which auth provider?" \
  -a "Use Auth0" \
  -s sprint-01 \
  -t auth -t security

# Update existing entry
llm-clarification add-clarification \
  -f tracking.yaml \
  --id CLR-001 \
  -a "Updated: Use Tailwind v4"

# Check for duplicates before adding
llm-clarification add-clarification \
  -f tracking.yaml \
  -q "CSS framework choice?" \
  -a "Tailwind" \
  --check-match
```

---

### list-entries

List all clarification entries with optional filters.

```bash
llm-clarification list-entries [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `--status` | Filter by status: pending, promoted, dismissed |
| `--min-occurrences N` | Filter by minimum occurrence count |
| `--json` | Output as JSON |

**Examples:**
```bash
# List all entries
llm-clarification list-entries -f tracking.yaml

# List only pending entries
llm-clarification list-entries -f tracking.yaml --status pending

# List frequently asked (3+ times)
llm-clarification list-entries -f tracking.yaml --min-occurrences 3

# JSON output
llm-clarification list-entries -f tracking.yaml --json
```

---

### promote-clarification

Promote a clarification entry to a target file (default: CLAUDE.md).

```bash
llm-clarification promote-clarification [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `--id` | Entry ID to promote (required) |
| `--target` | Target file for promotion (default: CLAUDE.md) |
| `--force` | Force re-promotion of already promoted entry |

**Examples:**
```bash
# Promote to default CLAUDE.md
llm-clarification promote-clarification \
  -f tracking.yaml \
  --id CLR-001

# Promote to custom file
llm-clarification promote-clarification \
  -f tracking.yaml \
  --id CLR-001 \
  --target docs/decisions.md

# Force re-promote
llm-clarification promote-clarification \
  -f tracking.yaml \
  --id CLR-001 \
  --force
```

**Effect:**
1. Appends the clarification Q&A to the target file
2. Updates the entry status to "promoted" in the tracking file

---

### delete-clarification

Delete a clarification entry from storage.

```bash
llm-clarification delete-clarification [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Storage file path (required) |
| `--id` | Entry ID to delete (required) |
| `--force` | Skip confirmation prompt |
| `-q, --quiet` | Suppress output |

**Examples:**
```bash
# Delete with confirmation
llm-clarification delete-clarification \
  -f tracking.yaml \
  --id CLR-001

# Delete without confirmation
llm-clarification delete-clarification \
  -f tracking.db \
  --id CLR-001 \
  --force

# Silent delete for scripting
llm-clarification delete-clarification \
  -f tracking.db \
  --id CLR-001 \
  --force --quiet
```

---

## Storage Commands

### export-memory

Export clarifications from any storage backend to YAML for editing or backup.

```bash
llm-clarification export-memory [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-s, --source` | Source storage file (required) |
| `-o, --output` | Output YAML file (required) |
| `-q, --quiet` | Suppress output |

**Examples:**
```bash
# Export SQLite to YAML
llm-clarification export-memory \
  --source clarifications.db \
  --output backup.yaml

# Export for editing
llm-clarification export-memory \
  --source data.db \
  --output editable.yaml
```

---

### import-memory

Import clarifications from YAML into any storage backend.

```bash
llm-clarification import-memory [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-s, --source` | Source YAML file (required) |
| `-t, --target` | Target storage file (required) |
| `-m, --mode` | Import mode: append, overwrite, merge (default: append) |
| `-q, --quiet` | Suppress output |

**Import Modes:**
- `append` - Add new entries, skip existing IDs
- `overwrite` - Replace all data with source
- `merge` - Add new entries, update existing ones

**Examples:**
```bash
# Migrate YAML to SQLite
llm-clarification import-memory \
  --source clarifications.yaml \
  --target clarifications.db

# Merge updates
llm-clarification import-memory \
  --source updates.yaml \
  --target data.db \
  --mode merge

# Full replacement
llm-clarification import-memory \
  --source new-data.yaml \
  --target data.db \
  --mode overwrite
```

---

### optimize-memory

Optimize SQLite storage (vacuum, prune stale entries, show stats).

```bash
llm-clarification optimize-memory [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Storage file path (required) |
| `--vacuum` | Run SQLite VACUUM to reclaim space |
| `--prune-stale` | Remove entries older than duration (e.g., 30d, 90d) |
| `--stats` | Show storage statistics |
| `-q, --quiet` | Suppress output |

**Examples:**
```bash
# Show storage statistics
llm-clarification optimize-memory \
  -f data.db \
  --stats

# Vacuum database
llm-clarification optimize-memory \
  -f data.db \
  --vacuum

# Remove entries older than 90 days
llm-clarification optimize-memory \
  -f data.db \
  --prune-stale 90d

# Combined optimization
llm-clarification optimize-memory \
  -f data.db \
  --vacuum --prune-stale 30d --stats
```

---

### reconcile-memory

Reconcile clarifications against the current codebase, identifying stale file references.

```bash
llm-clarification reconcile-memory [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Storage file path (required) |
| `-p, --project-root` | Project root directory (required) |
| `--dry-run` | Show changes without applying |
| `-q, --quiet` | Suppress output |

**Examples:**
```bash
# Check for stale references (dry run)
llm-clarification reconcile-memory \
  -f tracking.db \
  -p /path/to/project \
  --dry-run

# Apply reconciliation
llm-clarification reconcile-memory \
  -f tracking.db \
  -p .
```

**Effect:**
1. Scans clarifications for file path references
2. Identifies references to files that no longer exist
3. Marks or removes stale references

---

## Analysis Commands (Require API)

### match-clarification

Find if a question matches any existing clarifications using LLM semantic matching.

```bash
llm-clarification match-clarification [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `-q, --question` | Question to match (required) |

**Examples:**
```bash
llm-clarification match-clarification \
  -f tracking.yaml \
  -q "What CSS framework should we use?"

llm-clarification match-clarification \
  -f tracking.yaml \
  -q "How do we handle authentication?"
```

**Output:**
```
MATCH_ID: CLR-001
CONFIDENCE: 0.85
REASONING: Both questions ask about CSS/styling framework choice
EXISTING_QUESTION: Should we use Tailwind or CSS modules?
EXISTING_ANSWER: Use Tailwind for this project
```

---

### cluster-clarifications

Group semantically similar questions into clusters.

```bash
llm-clarification cluster-clarifications [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |

**Examples:**
```bash
llm-clarification cluster-clarifications -f tracking.yaml
```

**Output:**
```
CLUSTER 1: Styling Decisions
  - Should we use Tailwind or CSS modules?
  - What CSS framework to use?
  - How should we handle component styling?

CLUSTER 2: Authentication
  - Which auth provider should we use?
  - How do we handle user authentication?

UNCLUSTERED:
  - What database should we use?
```

Useful for identifying duplicate or related clarifications across sprints.

---

### detect-conflicts

Find clarification entries that may have conflicting answers.

```bash
llm-clarification detect-conflicts [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |

**Examples:**
```bash
llm-clarification detect-conflicts -f tracking.yaml
```

**Output:**
```
CONFLICT DETECTED:
  Entry 1 (CLR-001): "Use Tailwind for styling"
  Entry 2 (CLR-015): "Use CSS modules for component isolation"
  Reason: Both address CSS/styling approach with contradictory answers

CONFLICT DETECTED:
  Entry 1 (CLR-003): "Use PostgreSQL"
  Entry 2 (CLR-022): "Use MongoDB for flexibility"
  Reason: Database choice conflict
```

---

### validate-clarifications

Check for stale or outdated entries based on project context.

```bash
llm-clarification validate-clarifications [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `-c, --context` | Project context (optional, auto-detected) |

**Examples:**
```bash
# Auto-detect project context
llm-clarification validate-clarifications -f tracking.yaml

# Provide explicit context
llm-clarification validate-clarifications \
  -f tracking.yaml \
  -c "React 18 project with TypeScript and Prisma"
```

**Output:**
```
POTENTIALLY STALE:
  CLR-005: "Use React 17 class components"
    Reason: Project now uses React 18 with hooks

STILL VALID:
  CLR-001: "Use Tailwind for styling"
  CLR-003: "Use PostgreSQL for database"
```

---

## Optimization Commands (Require API)

### normalize-clarification

Use an LLM to improve and standardize question wording.

```bash
llm-clarification normalize-clarification [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-q, --question` | Question to normalize (required) |

**Examples:**
```bash
llm-clarification normalize-clarification \
  -q "so like what css thing should we use ya know"
```

**Output:**
```
ORIGINAL: so like what css thing should we use ya know
NORMALIZED: Which CSS framework or styling approach should we use for this project?
```

Useful for cleaning up informal or unclear questions before adding to tracking.

---

### suggest-consolidation

Identify similar clarifications that could be merged.

```bash
llm-clarification suggest-consolidation [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |

**Examples:**
```bash
llm-clarification suggest-consolidation -f tracking.yaml
```

**Output:**
```
CONSOLIDATION SUGGESTION 1:
  Entries: CLR-001, CLR-015, CLR-023
  Topic: CSS/Styling framework choice
  Suggested merged question: "Which CSS framework should we use?"
  Suggested answer: "Use Tailwind CSS for all styling"

CONSOLIDATION SUGGESTION 2:
  Entries: CLR-003, CLR-007
  Topic: Database technology
  Suggested merged question: "What database should we use?"
  Suggested answer: "Use PostgreSQL with Prisma ORM"
```

---

### identify-candidates

Find clarifications that should be promoted to permanent documentation.

```bash
llm-clarification identify-candidates [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-f, --file` | Tracking file path (required) |
| `--min-occurrences N` | Minimum occurrences to consider (default: 3) |

**Examples:**
```bash
# Default (3+ occurrences)
llm-clarification identify-candidates -f tracking.yaml

# Lower threshold
llm-clarification identify-candidates \
  -f tracking.yaml \
  --min-occurrences 2
```

**Output:**
```
PROMOTION CANDIDATES:

HIGH PRIORITY:
  CLR-001: "Use Tailwind for styling" (5 occurrences)
    Reason: Foundational architecture decision, frequently referenced

  CLR-003: "Use PostgreSQL" (4 occurrences)
    Reason: Core infrastructure choice, affects many components

MEDIUM PRIORITY:
  CLR-007: "Use React Query for data fetching" (3 occurrences)
    Reason: Common implementation pattern
```

---

## Workflow Examples

### Starting a New Project

```bash
# 1. Initialize tracking
llm-clarification init-tracking -o .planning/clarifications.yaml

# 2. Add clarifications as they arise
llm-clarification add-clarification \
  -f .planning/clarifications.yaml \
  -q "Which testing framework?" \
  -a "Use Vitest for unit tests, Playwright for e2e" \
  -s sprint-01 \
  -t testing

# 3. Check for duplicates before asking again
llm-clarification match-clarification \
  -f .planning/clarifications.yaml \
  -q "What should we use for testing?"
```

### Sprint Review

```bash
# 1. List all entries from this sprint
llm-clarification list-entries -f tracking.yaml --status pending

# 2. Find conflicts
llm-clarification detect-conflicts -f tracking.yaml

# 3. Identify promotion candidates
llm-clarification identify-candidates -f tracking.yaml

# 4. Promote important decisions
llm-clarification promote-clarification \
  -f tracking.yaml \
  --id CLR-001 \
  --target CLAUDE.md
```

### Maintenance

```bash
# 1. Find stale entries
llm-clarification validate-clarifications -f tracking.yaml

# 2. Cluster similar questions
llm-clarification cluster-clarifications -f tracking.yaml

# 3. Suggest consolidations
llm-clarification suggest-consolidation -f tracking.yaml
```

---

## Best Practices

1. **Add clarifications immediately** - Capture decisions as they're made
2. **Use context tags** - Makes filtering and clustering more effective
3. **Check for matches first** - Avoid duplicate entries with `--check-match`
4. **Review periodically** - Run `validate-clarifications` each sprint
5. **Promote frequently** - Move stable decisions to permanent docs
6. **Clean up conflicts** - Resolve contradictory answers promptly

---

## See Also

- [README.md](../README.md) - Main documentation
- [quick-reference.md](quick-reference.md) - Command cheat sheet
- [MCP_SETUP.md](MCP_SETUP.md) - Claude Desktop integration
- [llm-support-commands.md](llm-support-commands.md) - llm-support reference
