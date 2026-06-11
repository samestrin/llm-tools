package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Sprint-plan scoping shared by the review fan-out commands (multi_review and
// review_direct). The --sprint-plan flag points at a sprint-plan.md or epic
// file whose content is injected into reviewer prompts so findings stay
// scoped to the sprint's work items.

// readSprintPlan reads the sprint plan file at path. A missing file is
// silently ignored (the caller's skill checks existence first; reviewers fall
// back to diff-touched-line scoping). Any other read error emits a warning to
// errW and the review continues unscoped.
func readSprintPlan(path string, errW io.Writer) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(errW, "warning: could not read --sprint-plan %s: %v\n", path, err)
		}
		return ""
	}
	// A blank plan would inject a scope block with zero in-scope items,
	// instructing reviewers to suppress every finding. Treat it as no plan.
	if strings.TrimSpace(string(data)) == "" {
		return ""
	}
	return string(data)
}

// sprintPlanScopeBlock renders the SCOPE CONSTRAINT prompt section for a
// sprint plan. Returns "" when no sprint plan was provided.
func sprintPlanScopeBlock(sprintPlan string) string {
	if strings.TrimSpace(sprintPlan) == "" {
		return ""
	}
	return `
SCOPE CONSTRAINT (from sprint-plan.md):

The following sprint plan defines what is IN SCOPE for this review. Only flag
issues in files/areas directly related to the work items below. Do NOT flag
issues in unrelated code that happens to appear in the diff (e.g., unrelated
refactoring, formatting changes, or dependencies pulled in by the changes).

--- BEGIN SPRINT PLAN ---
` + sprintPlan + `
--- END SPRINT PLAN ---

Apply this scope: if a finding is in a file not mentioned in the sprint plan's
tasks/stories, or addresses concerns outside the sprint's stated goals, mark it
as OUT-OF-SCOPE in your review (do not include it in TD_STREAM).
`
}
