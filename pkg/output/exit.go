package output

import (
	"strings"

	goaxi "github.com/samestrin/go-axi"
)

// usageMarkers are the fragments cobra puts in an error when the INVOCATION is
// malformed, as opposed to when the command ran and failed.
//
// Every one was captured from cobra v1.8.0 itself — the version this module
// pins — rather than written from memory:
//
//	unknown flag: --bogus
//	unknown shorthand flag: 'Z' in -Z
//	unknown command "nosuch" for "probe"
//	flag needs an argument: --file
//	invalid argument "nope" for "--depth" flag: strconv.ParseInt: ...
//	required flag(s) "file" not set
//
// This is a string match, and it is fragile by nature: cobra exports no typed
// error for any of these. TestExitCodeForUsageErrors pins the exact wording so
// a cobra upgrade that rewords one breaks a test, instead of silently
// downgrading every usage error back to exit 1 with nobody noticing.
//
// They are matched as substrings rather than prefixes because a command may
// wrap the error on its way out; the fragments are distinctive enough that
// ordinary prose does not contain them.
var usageMarkers = []string{
	"unknown flag: ",
	"unknown shorthand flag: ",
	`unknown command "`,
	"flag needs an argument: ",
	`invalid argument "`,
	`required flag(s) "`,
}

// ExitCodeFor classifies err into the process status the AXI contract expects.
//
// The split matters to a caller that cannot see the message. Exit 1 says the
// tool itself failed and retrying may be worthwhile; exit 2 says the invocation
// was malformed and retrying it unchanged never will be. Collapsing the two —
// which is what all three CLIs did — leaves an agent unable to tell a typo in a
// flag name from a missing file.
func ExitCodeFor(err error) int {
	if err == nil {
		return int(goaxi.ExitOK)
	}
	msg := err.Error()
	for _, marker := range usageMarkers {
		if strings.Contains(msg, marker) {
			return int(goaxi.ExitUsage)
		}
	}
	return int(goaxi.ExitError)
}
