package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureTDValidateReadme is a realistic TD README with all checkbox variants.
//
// Open (non-deferred) [ ] rows (5 total, processed in open mode):
//  1. scripts/foo.py:42         — line ref, file exists              → valid
//  2. scripts/foo.py:run_func   — symbol, file exists, symbol found  → valid
//  3. scripts/foo.py:ghost_sym  — symbol, file exists, NOT found     → symbol_not_found
//  4. scripts/missing.py:helper — file does not exist                → file_missing
//  5. (empty file column)                                            → no_file
//
// Deferred rows (excluded in open mode, included in all mode):
//  6. [ ] with intent_note: deferred in Problem — scripts/foo.py:run_func → valid
//  7. [/] explicit deferred                     — scripts/foo.py:42       → valid
//
// Resolved (always excluded):
//  8. [x] scripts/foo.py:42
const fixtureTDValidateReadme = `# Technical Debt

### [2026-01-01] From Sprint: test_sprint

| Group | | Severity | File | Problem | Fix | Category | Est Minutes | Source | Reviewers | Confidence |
|-------|---|----------|------|---------|-----|----------|-------------|--------|-----------|------------|
| 1 | [ ] | MEDIUM | scripts/foo.py:42 | line ref item | fix | perf | 20 | test | alice | MEDIUM |
| 1 | [ ] | HIGH | scripts/foo.py:run_func | symbol found item | fix | bug | 30 | test | bob | HIGH |
| 2 | [ ] | HIGH | scripts/foo.py:ghost_sym | symbol not in file | fix | bug | 10 | test | carol | HIGH |
| 2 | [ ] | LOW | scripts/missing.py:helper | file does not exist | fix | bug | 15 | test | dave | LOW |
| 3 | [ ] | LOW |  | empty file column | fix | bug | 5 | test | eve | LOW |
| 3 | [ ] | MEDIUM | scripts/foo.py:run_func | (intent_note: deferred by design) item | fix | bug | 25 | test | frank | MEDIUM |
| 4 | [/] | LOW | scripts/foo.py:42 | explicit deferred | later | perf | 60 | test | grace | LOW |
| 4 | [x] | HIGH | scripts/foo.py:42 | resolved item | done | bug | 45 | test | henry | HIGH |
`

// writeTDValidateFiles creates a temp directory with:
//   - README.md containing content
//   - scripts/foo.py containing a def run_func(): stub
//
// Returns (readmePath, rootDir).
func writeTDValidateFiles(t *testing.T, content string) (readmePath, rootDir string) {
	t.Helper()
	rootDir = t.TempDir()
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fooContent := "# python stub\ndef run_func():\n    pass\n\nclass MyClass:\n    pass\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "foo.py"), []byte(fooContent), 0o644); err != nil {
		t.Fatal(err)
	}
	readmePath = filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func runTDValidateCmd(t *testing.T, args ...string) (TDValidateResult, string, error) {
	t.Helper()
	cmd := newTDValidateCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var res TDValidateResult
	if err == nil {
		if jerr := json.Unmarshal(out.Bytes(), &res); jerr != nil {
			t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out.String())
		}
	}
	return res, errb.String(), err
}

// TestTDValidate_OpenModeDefault verifies that the default (open) mode includes
// only [ ] rows without an intent_note deferred marker, and excludes [/] and [x].
func TestTDValidate_OpenModeDefault(t *testing.T) {
	readmePath, rootDir := writeTDValidateFiles(t, fixtureTDValidateReadme)
	res, stderr, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	if res.Summary.Total != 5 {
		t.Errorf("total = %d, want 5 (open non-deferred rows only)", res.Summary.Total)
	}
	if res.Summary.DeferredChecked != 0 {
		t.Errorf("deferred_checked = %d, want 0 in open mode", res.Summary.DeferredChecked)
	}
	if res.Summary.OpenChecked != 5 {
		t.Errorf("open_checked = %d, want 5", res.Summary.OpenChecked)
	}
	for _, item := range res.Items {
		if item.Checkbox == "[x]" || item.Checkbox == "[X]" {
			t.Errorf("resolved [x] row should never appear: %+v", item)
		}
		if item.Checkbox == "[/]" {
			t.Errorf("[/] row should not appear in open mode: %+v", item)
		}
	}
}

// TestTDValidate_AllModeIncludesDeferred verifies that --mode all includes [/] rows
// and [ ] rows with intent_note deferred, but still excludes [x].
func TestTDValidate_AllModeIncludesDeferred(t *testing.T) {
	readmePath, rootDir := writeTDValidateFiles(t, fixtureTDValidateReadme)
	res, stderr, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--mode", "all", "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	// 5 open + 1 intent_note deferred + 1 [/] deferred = 7; [x] always excluded
	if res.Summary.Total != 7 {
		t.Errorf("total = %d, want 7 (open + deferred, no resolved)", res.Summary.Total)
	}
	if res.Summary.DeferredChecked != 2 {
		t.Errorf("deferred_checked = %d, want 2", res.Summary.DeferredChecked)
	}
	if res.Summary.OpenChecked != 5 {
		t.Errorf("open_checked = %d, want 5", res.Summary.OpenChecked)
	}
	for _, item := range res.Items {
		if item.Checkbox == "[x]" || item.Checkbox == "[X]" {
			t.Errorf("resolved [x] row should never appear: %+v", item)
		}
	}
}

// TestTDValidate_FileExists_LineRef verifies that a line-ref entry (foo.py:42) with an
// existing file produces status=valid and symbol_found=null.
func TestTDValidate_FileExists_LineRef(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW | scripts/foo.py:42 | prob | fix | perf | 10 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Status != "valid" {
		t.Errorf("status = %q, want valid", item.Status)
	}
	if !item.FileExists {
		t.Errorf("file_exists should be true")
	}
	if item.SymbolFound != nil {
		t.Errorf("symbol_found should be null for line-ref, got %v", *item.SymbolFound)
	}
	if item.Symbol != "" {
		t.Errorf("symbol = %q, want empty for line-ref", item.Symbol)
	}
}

// TestTDValidate_FileExists_SymbolFound verifies that a symbol-ref with an existing file
// and matching symbol produces status=valid and symbol_found=true.
func TestTDValidate_FileExists_SymbolFound(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | HIGH | scripts/foo.py:run_func | prob | fix | bug | 10 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Status != "valid" {
		t.Errorf("status = %q, want valid", item.Status)
	}
	if item.Symbol != "run_func" {
		t.Errorf("symbol = %q, want run_func", item.Symbol)
	}
	if item.SymbolFound == nil || !*item.SymbolFound {
		t.Errorf("symbol_found should be true")
	}
}

// TestTDValidate_FileMissing verifies that a missing file produces status=file_missing.
func TestTDValidate_FileMissing(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW | scripts/missing.py:helper | prob | fix | bug | 10 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Status != "file_missing" {
		t.Errorf("status = %q, want file_missing", item.Status)
	}
	if item.FileExists {
		t.Errorf("file_exists should be false")
	}
	if res.Summary.FileMissing != 1 {
		t.Errorf("file_missing count = %d, want 1", res.Summary.FileMissing)
	}
}

// TestTDValidate_SymbolNotFound verifies that an existing file without the cited
// symbol produces status=symbol_not_found and symbol_found=false.
func TestTDValidate_SymbolNotFound(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | HIGH | scripts/foo.py:ghost_sym | prob | fix | bug | 10 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Status != "symbol_not_found" {
		t.Errorf("status = %q, want symbol_not_found", item.Status)
	}
	if item.SymbolFound == nil || *item.SymbolFound {
		t.Errorf("symbol_found should be false")
	}
	if res.Summary.SymbolNotFound != 1 {
		t.Errorf("symbol_not_found count = %d, want 1", res.Summary.SymbolNotFound)
	}
}

// TestTDValidate_ParseFileLine_Variants exercises parseFileLine with all real-world
// format variants observed in the yamtorah TD README.
func TestTDValidate_ParseFileLine_Variants(t *testing.T) {
	cases := []struct {
		input      string
		wantFile   string
		wantSymbol string
		wantOK     bool
	}{
		// Standard line reference
		{"scripts/foo.py:72", "scripts/foo.py", "", true},
		// Symbol reference
		{"scripts/foo.py:run_compilation", "scripts/foo.py", "run_compilation", true},
		// Symbol with multi-function slash notation — take first
		{"scripts/foo.py:run_compilation / _persist", "scripts/foo.py", "run_compilation", true},
		// File-only with parenthetical annotation — strip annotation
		{"scripts/foo.py (some note)", "scripts/foo.py", "", true},
		// Line range reference
		{"scripts/foo.py:289-322", "scripts/foo.py", "", true},
		// Multi-file slash notation — take first file
		{"scripts/a.py / scripts/b.py", "scripts/a.py", "", true},
		// Deep path with symbol
		{"scripts/wiki_compiler/article_generator.py:write", "scripts/wiki_compiler/article_generator.py", "write", true},
		// Backend path with line range
		{"backend/app/search/orchestrator.py:3568-3586", "backend/app/search/orchestrator.py", "", true},
		// Colon with empty suffix
		{"scripts/foo.py:", "scripts/foo.py", "", true},
		// Empty string
		{"", "", "", false},
		// Whitespace only
		{"   ", "", "", false},
		// File-only, no colon
		{"scripts/foo.py", "scripts/foo.py", "", true},
		// Symbol with multi-file then slash-notation — take first file, first symbol
		{"scripts/compile_wiki.py:run_compilation / _persist", "scripts/compile_wiki.py", "run_compilation", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			gotFile, gotSym, gotOK := parseFileLine(tc.input)
			if gotOK != tc.wantOK {
				t.Errorf("ok = %v, want %v", gotOK, tc.wantOK)
			}
			if gotFile != tc.wantFile {
				t.Errorf("file = %q, want %q", gotFile, tc.wantFile)
			}
			if gotSym != tc.wantSymbol {
				t.Errorf("symbol = %q, want %q", gotSym, tc.wantSymbol)
			}
		})
	}
}

// TestTDValidate_EmptyFileLine verifies that a blank File column produces status=no_file.
func TestTDValidate_EmptyFileLine(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW |  | empty file | fix | bug | 5 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Status != "no_file" {
		t.Errorf("status = %q, want no_file", res.Items[0].Status)
	}
	if res.Summary.NoFile != 1 {
		t.Errorf("no_file count = %d, want 1", res.Summary.NoFile)
	}
}

// TestTDValidate_IntentNoteDeferred_OpenMode verifies that a [ ] row whose Problem
// contains "(intent_note: deferred" is skipped in open mode but included in all mode.
func TestTDValidate_IntentNoteDeferred_OpenMode(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | MEDIUM | scripts/foo.py:run_func | (intent_note: deferred by design) | fix | bug | 25 |
| 2 | [ ] | LOW | scripts/foo.py:42 | normal open item | fix | perf | 10 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)

	// Open mode: deferred intent_note row excluded
	resOpen, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if resOpen.Summary.Total != 1 {
		t.Errorf("open mode: total = %d, want 1 (only non-deferred row)", resOpen.Summary.Total)
	}
	if resOpen.Summary.DeferredChecked != 0 {
		t.Errorf("open mode: deferred_checked = %d, want 0", resOpen.Summary.DeferredChecked)
	}

	// All mode: intent_note deferred row included
	resAll, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--mode", "all", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if resAll.Summary.Total != 2 {
		t.Errorf("all mode: total = %d, want 2", resAll.Summary.Total)
	}
	if resAll.Summary.DeferredChecked != 1 {
		t.Errorf("all mode: deferred_checked = %d, want 1", resAll.Summary.DeferredChecked)
	}
}

// TestTDValidate_Summary_Totals uses the full fixture to verify every summary field.
func TestTDValidate_Summary_Totals(t *testing.T) {
	readmePath, rootDir := writeTDValidateFiles(t, fixtureTDValidateReadme)
	res, stderr, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--mode", "all", "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	s := res.Summary
	// all mode: 7 items (5 open + 2 deferred); [x] always excluded
	if s.Total != 7 {
		t.Errorf("total = %d, want 7", s.Total)
	}
	// valid: rows 1 (foo.py:42), 2 (foo.py:run_func), 6 (intent_note, foo.py:run_func), 7 ([/] foo.py:42)
	if s.Valid != 4 {
		t.Errorf("valid = %d, want 4", s.Valid)
	}
	// file_missing: row 4 (scripts/missing.py)
	if s.FileMissing != 1 {
		t.Errorf("file_missing = %d, want 1", s.FileMissing)
	}
	// symbol_not_found: row 3 (foo.py:ghost_sym)
	if s.SymbolNotFound != 1 {
		t.Errorf("symbol_not_found = %d, want 1", s.SymbolNotFound)
	}
	// no_file: row 5 (empty column)
	if s.NoFile != 1 {
		t.Errorf("no_file = %d, want 1", s.NoFile)
	}
	if s.OpenChecked != 5 {
		t.Errorf("open_checked = %d, want 5", s.OpenChecked)
	}
	if s.DeferredChecked != 2 {
		t.Errorf("deferred_checked = %d, want 2", s.DeferredChecked)
	}
}

// TestTDValidate_JSONOutput verifies that --json produces valid JSON with a non-null items array.
func TestTDValidate_JSONOutput(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	cmd := newTDValidateCmd()
	cmd.SetArgs([]string{"--path", readmePath, "--root", rootDir, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var res TDValidateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	// items must not be JSON null even when empty
	if res.Items == nil {
		t.Error("items must not be JSON null (should be [])")
	}
}

// TestTDValidate_RootFlag verifies that --root resolves relative file paths correctly.
func TestTDValidate_RootFlag(t *testing.T) {
	rootDir := t.TempDir()
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "bar.py"), []byte("def myfunc(): pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW | scripts/bar.py:myfunc | prob | fix | bug | 5 |
`
	readmePath := filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}

	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Status != "valid" {
		t.Errorf("status = %q, want valid (--root resolved path correctly)", item.Status)
	}
	if item.SymbolFound == nil || !*item.SymbolFound {
		t.Errorf("symbol_found should be true")
	}
}

// TestTDValidate_SymbolFoundAfterLongLine verifies that a symbol present on a line
// following a very long line (>64KB default scanner buffer) is still found.
func TestTDValidate_SymbolFoundAfterLongLine(t *testing.T) {
	rootDir := t.TempDir()
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file with a 70KB first line followed by the symbol on the next line.
	longLine := strings.Repeat("x", 70*1024)
	content := longLine + "\ndef deep_func(): pass\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "big.py"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW | scripts/big.py:deep_func | prob | fix | bug | 5 |
`
	readmePath := filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Status != "valid" {
		t.Errorf("status = %q, want valid (symbol after long line must be found)", res.Items[0].Status)
	}
}

// TestTDValidate_MissingPath verifies that omitting --path returns an error.
func TestTDValidate_MissingPath(t *testing.T) {
	cmd := newTDValidateCmd()
	cmd.SetArgs([]string{"--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing --path must return an error")
	}
}

// TestTDValidate_BadMode verifies that an invalid --mode value returns an error.
func TestTDValidate_BadMode(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	_, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--mode", "bogus", "--json")
	if err == nil {
		t.Fatal("invalid --mode must return an error")
	}
}

// TestTDValidate_TextOutput verifies default (non-JSON) text output includes summary line
// and lists problematic items.
func TestTDValidate_TextOutput(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | LOW | scripts/foo.py:42 | ok | fix | perf | 10 |
| 2 | [ ] | HIGH | scripts/missing.py:help | gone | fix | bug | 5 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)
	cmd := newTDValidateCmd()
	cmd.SetArgs([]string{"--path", readmePath, "--root", rootDir})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "td_validate:") {
		t.Errorf("text output missing summary line, got:\n%s", text)
	}
	if !strings.Contains(text, "FILE_MISSING") {
		t.Errorf("text output missing FILE_MISSING line, got:\n%s", text)
	}
}

// TestTDValidate_ResolvedRowsAlwaysSkipped verifies [x] rows are excluded from all modes.
func TestTDValidate_ResolvedRowsAlwaysSkipped(t *testing.T) {
	readme := `# TD

### [2026-01-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [x] | HIGH | scripts/foo.py:42 | resolved | done | bug | 10 |
| 1 | [X] | LOW | scripts/foo.py:42 | also resolved | done | perf | 5 |
`
	readmePath, rootDir := writeTDValidateFiles(t, readme)

	for _, mode := range []string{"open", "all"} {
		res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--mode", mode, "--json")
		if err != nil {
			t.Fatalf("mode=%s: %v", mode, err)
		}
		if res.Summary.Total != 0 {
			t.Errorf("mode=%s: total = %d, want 0 (all resolved)", mode, res.Summary.Total)
		}
		if len(res.Items) != 0 {
			t.Errorf("mode=%s: got %d items, want 0", mode, len(res.Items))
		}
	}
}
