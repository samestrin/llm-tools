// Package gitrange deterministically resolves a git review range (base..head)
// for code-review fan-out commands. It handles three modes:
//
//   - merge-commit: --merge-commit <sha> → base = sha^, head = sha. Works for
//     true merge commits (first parent = mainline) AND single-parent squash
//     commits — the escape hatch for reviewing a branch that is already merged.
//   - explicit: --base (and optional --head) → rev-parse both.
//   - merge-base: neither given → merge-base of HEAD against the repository's
//     default branch (origin/HEAD, then main/master/origin/main/origin/master).
//
// All resolution is local git; no network, no shell interpolation.
package gitrange

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Params selects the resolution mode. MergeCommit is mutually exclusive with
// BaseRef/HeadRef.
type Params struct {
	RepoPath    string // default "."
	BaseRef     string // optional explicit base
	HeadRef     string // optional, default "HEAD"
	MergeCommit string // optional; base = sha^, head = sha
}

// Result is the resolved range. Base/Head are full SHAs. Empty means the range
// contains no commits (branch already merged, or head behind base); Message
// carries human guidance in that case.
type Result struct {
	Base         string `json:"base"`
	Head         string `json:"head"`
	BaseSymbolic string `json:"base_ref,omitempty"`
	CommitCount  int    `json:"commit_count"`
	FilesChanged int    `json:"files_changed"`
	Empty        bool   `json:"empty"`
	Detection    string `json:"detection"` // "merge-base" | "merge-commit" | "explicit"
	Message      string `json:"message,omitempty"`
}

// execGit is swappable for error-path tests.
var execGit = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// gitOut runs git and returns trimmed stdout.
func gitOut(dir string, args ...string) (string, error) {
	out, err := execGit(dir, args...)
	return strings.TrimSpace(out), err
}

// resolveCommit resolves a ref to a full commit SHA.
func resolveCommit(repo, ref string) (string, error) {
	sha, err := gitOut(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil || sha == "" {
		return "", fmt.Errorf("cannot resolve %q to a commit in %s", ref, repo)
	}
	return sha, nil
}

// Short returns the 7-char abbreviation of a SHA (safe on shorter strings).
func Short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Resolve computes the review range per Params. See package doc for modes.
func Resolve(p Params) (Result, error) {
	if p.MergeCommit != "" && (p.BaseRef != "" || p.HeadRef != "") {
		return Result{}, fmt.Errorf("--merge-commit is mutually exclusive with --base/--head")
	}
	repo := p.RepoPath
	if repo == "" {
		repo = "."
	}
	if _, err := gitOut(repo, "rev-parse", "--git-dir"); err != nil {
		return Result{}, fmt.Errorf("not a git repository: %s", repo)
	}

	var res Result
	switch {
	case p.MergeCommit != "":
		sha, err := resolveCommit(repo, p.MergeCommit)
		if err != nil {
			return Result{}, fmt.Errorf("cannot resolve merge commit: %w", err)
		}
		parent, err := gitOut(repo, "rev-parse", "--verify", "--quiet", sha+"^")
		if err != nil || parent == "" {
			return Result{}, fmt.Errorf("merge commit %s has no parent (root commit) — cannot derive a base", Short(sha))
		}
		res = Result{Base: parent, Head: sha, Detection: "merge-commit"}

	case p.BaseRef != "":
		head := p.HeadRef
		if head == "" {
			head = "HEAD"
		}
		baseSHA, err := resolveCommit(repo, p.BaseRef)
		if err != nil {
			return Result{}, err
		}
		headSHA, err := resolveCommit(repo, head)
		if err != nil {
			return Result{}, err
		}
		res = Result{Base: baseSHA, Head: headSHA, Detection: "explicit"}

	default:
		head := p.HeadRef
		if head == "" {
			head = "HEAD"
		}
		headSHA, err := resolveCommit(repo, head)
		if err != nil {
			return Result{}, err
		}
		defBranch, err := defaultBranch(repo)
		if err != nil {
			return Result{}, err
		}
		base, err := gitOut(repo, "merge-base", defBranch, headSHA)
		if err != nil {
			if isShallow(repo) {
				return Result{}, fmt.Errorf("shallow clone: merge-base against %s unavailable; run `git fetch --unshallow` or pass --base explicitly", defBranch)
			}
			return Result{}, fmt.Errorf("merge-base %s %s failed: %w", defBranch, Short(headSHA), err)
		}
		res = Result{Base: base, Head: headSHA, BaseSymbolic: defBranch, Detection: "merge-base"}
	}

	countStr, err := gitOut(repo, "rev-list", "--count", res.Base+".."+res.Head)
	if err != nil {
		return Result{}, fmt.Errorf("rev-list --count failed: %w", err)
	}
	res.CommitCount, err = strconv.Atoi(countStr)
	if err != nil {
		return Result{}, fmt.Errorf("unexpected rev-list output %q", countStr)
	}

	names, err := gitOut(repo, "diff", "--name-only", res.Base+".."+res.Head)
	if err != nil {
		return Result{}, fmt.Errorf("diff --name-only failed: %w", err)
	}
	if names != "" {
		res.FilesChanged = len(strings.Split(names, "\n"))
	}

	res.Empty = res.Base == res.Head || res.CommitCount == 0
	if res.Empty {
		res.Message = fmt.Sprintf(
			"Empty range: %s..%s contains no commits. The branch may already be merged into the default branch, or head is behind base. "+
				"Re-run with --merge-commit <sha-of-the-merge>, or pass explicit --base/--head.",
			Short(res.Base), Short(res.Head))
	}
	return res, nil
}

// defaultBranch determines the repository's default branch: origin/HEAD first
// (handles non-main/master defaults), then common candidates.
func defaultBranch(repo string) (string, error) {
	if ref, err := gitOut(repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && ref != "" {
		if _, err := resolveCommit(repo, ref); err == nil {
			return ref, nil
		}
	}
	// Remote-tracking refs first: when origin exists, origin/main is the
	// upstream truth — a stale local main would over-widen the range.
	for _, cand := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := resolveCommit(repo, cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("cannot determine default branch (tried origin/HEAD, origin/main, origin/master, main, master); pass --base explicitly")
}

func isShallow(repo string) bool {
	out, err := gitOut(repo, "rev-parse", "--is-shallow-repository")
	return err == nil && out == "true"
}

// BundleReachable reports whether sha is reachable from HEAD, local branches,
// or tags — the refs `git bundle create HEAD --branches --tags` ships. A
// commit that is only reachable from remote-tracking refs (e.g. a squash
// merge fetched but not pulled) resolves locally yet would be missing from
// the bundle.
func BundleReachable(repoPath, sha string) (bool, error) {
	out, err := gitOut(repoPath, "rev-list", "--max-count=1", sha, "--not", "HEAD", "--branches", "--tags")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// DefaultExcludeGlobs is the built-in set of path globs dropped from review
// diffs unless overridden: notes and tracking artifacts that are noise (or
// actively misleading) to a code reviewer — the technical-debt README reads
// like reviewer output, CHANGELOG churn carries no reviewable logic, and
// documentation directories hold prose rather than code.
var DefaultExcludeGlobs = []string{".planning/**", "CHANGELOG.md", "doc*/**"}

// excludePathspec converts exclude globs into the trailing argv git needs to
// drop them from a diff: a positive ":(top)" pathspec (required — exclude-only
// pathspecs match nothing) followed by one ":(top,exclude)<glob>" per glob. The
// "top" magic makes every pathspec repo-root-relative, so the diff covers the
// whole repository (matching a bare `git diff`) and the globs anchor at the repo
// root regardless of the process cwd — a cwd-relative "." would wrongly scope
// the diff to a subdirectory when repoPath is not the repo root. Returns nil for
// an empty/nil list so the no-exclude path stays byte-identical to `git diff`.
func excludePathspec(globs []string) []string {
	if len(globs) == 0 {
		return nil
	}
	args := make([]string, 0, len(globs)+2)
	args = append(args, "--", ":(top)")
	for _, g := range globs {
		args = append(args, ":(top,exclude)"+g)
	}
	return args
}

// Diff returns the unified diff text for base..head. Raw output is preserved
// (no trimming) so callers can write it verbatim to a diff file.
func Diff(repoPath, base, head string) (string, error) {
	return DiffExcluding(repoPath, base, head, nil)
}

// DiffExcluding returns the unified diff for base..head with any paths matching
// the exclude globs dropped (git ":(exclude)" pathspecs). A nil/empty excludes
// list yields output byte-identical to Diff. Globs reach git as argv elements
// via exec.Command (no shell), so no quoting/injection concern applies.
func DiffExcluding(repoPath, base, head string, excludes []string) (string, error) {
	args := append([]string{"diff", base + ".." + head}, excludePathspec(excludes)...)
	return execGit(repoPath, args...)
}

// ExcludedFileNames returns the changed files in base..head that the exclude
// globs drop — the set difference between the full changed-file list and the
// list after applying the same ":(exclude)" pathspecs. Using identical
// pathspecs guarantees the report matches exactly what DiffExcluding omitted.
// Returns an empty slice (never nil) when excludes is empty.
func ExcludedFileNames(repoPath, base, head string, excludes []string) ([]string, error) {
	if len(excludes) == 0 {
		return []string{}, nil
	}
	all, err := changedNameSet(repoPath, base, head, nil)
	if err != nil {
		return nil, err
	}
	kept, err := changedNameSet(repoPath, base, head, excludes)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for name := range all {
		if !kept[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// changedNameSet returns the set of changed file names in base..head, optionally
// with exclude pathspecs applied.
func changedNameSet(repoPath, base, head string, excludes []string) (map[string]bool, error) {
	args := append([]string{"diff", "--name-only", base + ".." + head}, excludePathspec(excludes)...)
	out, err := gitOut(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("diff --name-only failed: %w", err)
	}
	set := make(map[string]bool)
	if out != "" {
		for _, name := range strings.Split(out, "\n") {
			if name != "" {
				set[name] = true
			}
		}
	}
	return set, nil
}
