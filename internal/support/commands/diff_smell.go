package commands

import (
	"regexp"
	"strings"
)

// Smell is one over-simplification fingerprint found in a diff.
type Smell struct {
	Type     string `json:"type"`     // test_only, weakened_assertion, suppression, empty_catch, stub_body
	Severity string `json:"severity"` // hard | soft
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"` // new-file line number for added-line smells
	Evidence string `json:"evidence"`
}

// SmellFiles lists the changed files split by role.
type SmellFiles struct {
	Test []string `json:"test"`
	Impl []string `json:"impl"`
}

// SmellSummary aggregates the scan.
type SmellSummary struct {
	TestFiles int            `json:"test_files"`
	ImplFiles int            `json:"impl_files"`
	Hard      int            `json:"hard"`
	Soft      int            `json:"soft"`
	ByType    map[string]int `json:"by_type,omitempty"`
	Verdict   string         `json:"verdict"` // hard | soft_only | clean
}

// SmellResult is the full diff-smell output.
type SmellResult struct {
	Files   SmellFiles   `json:"files"`
	Smells  []Smell      `json:"smells"`
	Summary SmellSummary `json:"summary"`
}

// --- detectors ---

var (
	goTestRe   = regexp.MustCompile(`_test\.go$`)
	jsTestRe   = regexp.MustCompile(`\.(test|spec)\.[cm]?[jt]sx?$`)
	pyTestRe   = regexp.MustCompile(`(^|/)(test_[^/]*\.py|[^/]*_test\.py)$`)
	rbTestRe   = regexp.MustCompile(`(^|/)[^/]*_(spec|test)\.rb$`)
	jvmTestRe  = regexp.MustCompile(`(Test|Tests|Spec)\.(java|kt|kts|scala)$`)
	csTestRe   = regexp.MustCompile(`(Test|Tests)\.cs$`)
	testSegSet = map[string]bool{"test": true, "tests": true, "spec": true, "__tests__": true}

	// An added line that suppresses a linter / type checker.
	suppressionRe = regexp.MustCompile(`(@ts-ignore|@ts-expect-error|eslint-disable|#\s*type:\s*ignore|#\s*noqa|#\s*pylint:\s*disable|#\s*pragma:\s*no\s*cover|#\s*nosec|//\s*nolint|@SuppressWarnings|#\s*rubocop:disable|@phpstan-ignore)`)

	// An added empty / swallowing exception handler.
	emptyCatchRe = regexp.MustCompile(`(except\b[^:]*:\s*pass\b|catch\s*(\([^)]*\))?\s*\{\s*\}|rescue\b[^;]*;\s*end\b)`)

	// An added stub / not-implemented / deferred body.
	stubBodyRe = regexp.MustCompile(`(?i)(panic\s*\(|raise\s+NotImplementedError|throw\s+new\s+Error\s*\(\s*["']not[ _]?implemented|\bTODO\b|\bFIXME\b)`)

	// A line that asserts something (used to detect weakened test assertions).
	assertionRe = regexp.MustCompile(`(?i)(\bassert\b|expect\s*\(|\.should\b|\.to(Be|Equal|Throw|Contain|Match|HaveBeen)|t\.(Error|Fatal|Errorf|Fatalf)|require\.\w|XCTAssert|EXPECT_|ASSERT_)`)
)

// isTestPath reports whether a repo-relative path is a test file. It is precise
// — "latest_test_results.go" or "contest.go" are NOT tests.
func isTestPath(p string) bool {
	p = strings.TrimPrefix(p, "./")
	for _, seg := range strings.Split(p, "/") {
		if testSegSet[seg] {
			return true
		}
	}
	return goTestRe.MatchString(p) || jsTestRe.MatchString(p) || pyTestRe.MatchString(p) ||
		rbTestRe.MatchString(p) || jvmTestRe.MatchString(p) || csTestRe.MatchString(p)
}

type addedLine struct {
	text   string
	lineNo int
}

type fileChange struct {
	path    string
	isTest  bool
	added   []addedLine
	removed []string
}

// analyzeDiff scans a unified diff for over-simplification fingerprints.
func analyzeDiff(diff string) *SmellResult {
	res := &SmellResult{
		Files:   SmellFiles{Test: []string{}, Impl: []string{}},
		Smells:  []Smell{},
		Summary: SmellSummary{ByType: map[string]int{}},
	}

	files := []*fileChange{}
	byPath := map[string]*fileChange{}
	var cur *fileChange
	newLineNo := 0

	ensure := func(p string) *fileChange {
		if p == "" || p == "/dev/null" {
			return nil
		}
		if fc, ok := byPath[p]; ok {
			return fc
		}
		fc := &fileChange{path: p, isTest: isTestPath(p)}
		byPath[p] = fc
		files = append(files, fc)
		return fc
	}

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			// "diff --git a/old b/new" — tentative file (overridden by +++).
			if p := bPathFromGitHeader(line); p != "" {
				cur = ensure(p)
			}
		case strings.HasPrefix(line, "+++ "):
			cur = ensure(headerPath(line[4:]))
		case strings.HasPrefix(line, "--- "):
			// old-file header — ignore
		case strings.HasPrefix(line, "@@"):
			newLineNo = newHunkStart(line)
		case strings.HasPrefix(line, "+"):
			if cur != nil {
				cur.added = append(cur.added, addedLine{text: line[1:], lineNo: newLineNo})
			}
			newLineNo++
		case strings.HasPrefix(line, "-"):
			if cur != nil {
				cur.removed = append(cur.removed, line[1:])
			}
		case strings.HasPrefix(line, "\\"):
			// "\ No newline at end of file" — ignore
		default:
			// context line (leading space) or stray line
			newLineNo++
		}
	}

	implCount, testCount := 0, 0
	for _, fc := range files {
		if fc.isTest {
			testCount++
			res.Files.Test = append(res.Files.Test, fc.path)
		} else {
			implCount++
			res.Files.Impl = append(res.Files.Impl, fc.path)
		}
	}
	res.Summary.TestFiles = testCount
	res.Summary.ImplFiles = implCount

	add := func(s Smell) {
		res.Smells = append(res.Smells, s)
		res.Summary.ByType[s.Type]++
		if s.Severity == "hard" {
			res.Summary.Hard++
		} else {
			res.Summary.Soft++
		}
	}

	// HARD: the fix touched only tests.
	if testCount > 0 && implCount == 0 {
		add(Smell{Type: "test_only", Severity: "hard", File: res.Files.Test[0],
			Evidence: "fix changed only test file(s); no implementation change"})
	}

	for _, fc := range files {
		// HARD: a test file removed more assertions than it added.
		if fc.isTest {
			removedAsserts, addedAsserts := 0, 0
			for _, r := range fc.removed {
				if assertionRe.MatchString(r) {
					removedAsserts++
				}
			}
			for _, a := range fc.added {
				if assertionRe.MatchString(a.text) {
					addedAsserts++
				}
			}
			if removedAsserts > addedAsserts {
				add(Smell{Type: "weakened_assertion", Severity: "hard", File: fc.path,
					Evidence: "test removed assertion(s) without replacing them"})
			}
		}

		// SOFT: per-added-line fingerprints.
		for _, a := range fc.added {
			if m := suppressionRe.FindString(a.text); m != "" {
				add(Smell{Type: "suppression", Severity: "soft", File: fc.path, Line: a.lineNo, Evidence: strings.TrimSpace(a.text)})
			}
			if emptyCatchRe.MatchString(a.text) {
				add(Smell{Type: "empty_catch", Severity: "soft", File: fc.path, Line: a.lineNo, Evidence: strings.TrimSpace(a.text)})
			}
			if stubBodyRe.MatchString(a.text) {
				add(Smell{Type: "stub_body", Severity: "soft", File: fc.path, Line: a.lineNo, Evidence: strings.TrimSpace(a.text)})
			}
		}
	}

	switch {
	case res.Summary.Hard > 0:
		res.Summary.Verdict = "hard"
	case res.Summary.Soft > 0:
		res.Summary.Verdict = "soft_only"
	default:
		res.Summary.Verdict = "clean"
	}
	return res
}

// headerPath strips the "a/" or "b/" prefix and any trailing tab metadata from a
// `--- ` / `+++ ` header path.
func headerPath(s string) string {
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "a/")
	s = strings.TrimPrefix(s, "b/")
	return s
}

// bPathFromGitHeader extracts the new path from "diff --git a/x b/y".
func bPathFromGitHeader(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	return headerPath(fields[len(fields)-1])
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)

// newHunkStart returns the new-file starting line of a hunk header, or 0.
func newHunkStart(line string) int {
	m := hunkRe.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	return n
}
