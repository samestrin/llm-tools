package output

import (
	"errors"
	"fmt"
	"testing"

	goaxi "github.com/samestrin/go-axi"
)

// AXI requires a malformed invocation to fail LOUD and distinctly: exit 2, not
// the exit 1 that means "the tool itself failed". An agent cannot decide
// whether retrying is worthwhile when a typo in a flag name is reported the
// same way as a missing file.
//
// All three CLIs exited 1 for both. The strings below are the ones cobra
// actually produces, captured from the binaries rather than guessed:
//
//	$ llm-support --bogus        Error: unknown flag: --bogus
//	$ llm-support bogus-sub      Error: unknown command "bogus-sub" for "llm-support"
//	$ llm-semantic --bogus       Error: unknown flag: --bogus
//
// Matching on that text is a string match, so it is pinned here. A cobra
// upgrade that rewords either message must break this test rather than quietly
// downgrade every usage error back to exit 1.

func TestExitCodeForUsageErrors(t *testing.T) {
	for name, err := range map[string]error{
		"unknown long flag":     errors.New("unknown flag: --bogus"),
		"unknown short flag":    errors.New("unknown shorthand flag: 'x' in -x"),
		"unknown command":       errors.New(`unknown command "bogus-sub" for "llm-support"`),
		"unknown semantic cmd":  errors.New(`unknown command "bogus-sub" for "llm-semantic"`),
		"flag needs argument":   errors.New("flag needs an argument: --file"),
		"invalid argument":      errors.New(`invalid argument "nope" for "--depth" flag: strconv.ParseInt: parsing "nope": invalid syntax`),
		"wrapped unknown flag":  fmt.Errorf("running command: %w", errors.New("unknown flag: --bogus")),
		"required flag missing": errors.New(`required flag(s) "file" not set`),
	} {
		t.Run(name, func(t *testing.T) {
			if got := ExitCodeFor(err); got != int(goaxi.ExitUsage) {
				t.Errorf("ExitCodeFor(%q) = %d, want %d (ExitUsage)",
					err, got, int(goaxi.ExitUsage))
			}
		})
	}
}

func TestExitCodeForRuntimeErrors(t *testing.T) {
	// A genuine failure stays exit 1. Widening the usage class until ordinary
	// errors fall into it would be worse than never having split them: the
	// caller would be told to fix its invocation when the tool actually broke.
	for name, err := range map[string]error{
		"missing file":     errors.New("open /nope.axi: no such file or directory"),
		"decode failure":   errors.New(`toon: line 1: field "a" is declared twice`),
		"permission":       errors.New("open /etc/shadow: permission denied"),
		"mentions a flag":  errors.New("the --json flag could not be applied to this result"),
		"mentions command": errors.New("command failed while writing output"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := ExitCodeFor(err); got != int(goaxi.ExitError) {
				t.Errorf("ExitCodeFor(%q) = %d, want %d (ExitError)",
					err, got, int(goaxi.ExitError))
			}
		})
	}
}

func TestExitCodeForNilIsSuccess(t *testing.T) {
	if got := ExitCodeFor(nil); got != int(goaxi.ExitOK) {
		t.Errorf("ExitCodeFor(nil) = %d, want %d", got, int(goaxi.ExitOK))
	}
}

func TestPrintErrorReturnsTheClassifiedCode(t *testing.T) {
	// PrintError is what every CLI passes to os.Exit, so the classification has
	// to reach the process status and not stop at a helper nobody calls.
	f := New(false, false, discard{})
	if got := f.PrintError(errors.New("unknown flag: --bogus")); got != int(goaxi.ExitUsage) {
		t.Errorf("PrintError(usage) = %d, want %d", got, int(goaxi.ExitUsage))
	}
	if got := f.PrintError(errors.New("open /nope: no such file or directory")); got != int(goaxi.ExitError) {
		t.Errorf("PrintError(runtime) = %d, want %d", got, int(goaxi.ExitError))
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
