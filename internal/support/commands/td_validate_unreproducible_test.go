package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// td-validate grounds a row against the code it cites. A "[-]" row is closed —
// nobody is going to act on it — so grounding it is noise: it reports
// file_missing for a path that was already ruled a misfire, and buries the open
// rows a caller is actually looking for.
//
// Resolved rows are skipped for the same reason. Unreproducible rows are the
// other closed state and were not.
func TestValidateSkipsUnreproducibleRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	content := `# Tech Debt

### [2026-08-01] From Sprint: s

| Group | | Severity | File | Problem | Fix |
|-------|---|----------|------|---------|-----|
| 1 | [-] | HIGH | does/not/exist/closed.py:1 | Could not reproduce | none |
| 2 | [x] | HIGH | does/not/exist/fixed.py:1 | Fixed | done |
| 3 | [ ] | HIGH | does/not/exist/open.py:1 | Still open | fix it |
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	rows := parseTDValidateRows(content)

	for _, row := range rows {
		if row.Checkbox == "[-]" {
			t.Errorf("an unreproducible row reached validation: %+v", row)
		}
		if row.Checkbox == "[x]" || row.Checkbox == "[X]" {
			t.Errorf("a resolved row reached validation: %+v", row)
		}
	}

	if len(rows) != 1 {
		t.Fatalf("want only the open row, got %d rows: %+v", len(rows), rows)
	}
	if rows[0].FileLine != "does/not/exist/open.py:1" {
		t.Errorf("want the open row, got %+v", rows[0])
	}
}
