package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// llm-semantic has no output choke point: 23 sites construct a JSON encoder
// inline across 8 files. Wiring --axi here means routing all of them through
// one helper FIRST, and only then registering the flag — because a persistent
// flag that some commands ignore is worse than no flag at all. A consumer
// parsing TOON gets JSON, with no error and exit 0.
//
// That ordering is not theoretical. In llm-support, a count-based audit
// reported yaml.go fully covered while `yaml pop` was still unguarded, and
// three further paths (both dry-run previews, the scalar-prefix return) were
// found only by a reviewer. Counting call sites is what failed; this test
// checks the source instead.

// helperFile is the only file allowed to construct a JSON encoder directly.
const helperFile = "axi.go"

func TestNoDirectJSONEncodersOutsideTheHelper(t *testing.T) {
	// Structural, deliberately. A behavioural test per command would need 23
	// invocations with 23 different fixture setups, and every one I forgot
	// would be a silent gap — exactly the failure this guards.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}

	var offenders []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == helperFile {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, "json.NewEncoder") {
				offenders = append(offenders,
					name+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
	}

	if len(offenders) > 0 {
		t.Errorf("%d emit site(s) still build their own encoder instead of "+
			"routing through %s, so --axi cannot reach them:\n  %s",
			len(offenders), helperFile, strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestCompactSitesStayCompact guards the variant that a mechanical migration
// would silently erase.
//
// Two sites emit COMPACT json — index_status.go's "index not found" and
// "embedder offline" branches build the encoder inline with no SetIndent,
// unlike the other 21. Routing them through an indenting helper would change
// their --json output, which this work guarantees it does not touch.
func TestEmitJSONHonoursTheIndentChoice(t *testing.T) {
	var indented, compact strings.Builder
	if err := emitJSON(&indented, map[string]any{"a": 1}, true); err != nil {
		t.Fatalf("indented: %v", err)
	}
	if err := emitJSON(&compact, map[string]any{"a": 1}, false); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if !strings.Contains(indented.String(), "\n  ") {
		t.Errorf("indent=true did not indent: %q", indented.String())
	}
	if strings.Contains(compact.String(), "\n  ") {
		t.Errorf("indent=false indented anyway: %q", compact.String())
	}
}
