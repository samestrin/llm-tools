package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	epicNumberDirs   []string
	epicNumberParent string
	epicNumberJSON   bool
)

// epicNumberRe matches a leading epic number: N, N.M, or N.M.P followed by a
// separator. Anything else in the directory (README.md, notes, dotfiles) is not
// an epic and is ignored rather than treated as an error.
var epicNumberRe = regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?[-_]`)

// EpicNumberResult is the payload returned to callers.
type EpicNumberResult struct {
	Next   string   `json:"next"`
	Parent string   `json:"parent,omitempty"`
	InUse  int      `json:"in_use"`
	Dirs   []string `json:"dirs"`
}

// epicNum is a parsed epic number. Absent levels are -1 so 3 and 3.0 stay
// distinguishable from 3.1 when deciding what a "child" means.
type epicNum struct {
	major, minor, patch int
}

func parseEpicNum(name string) (epicNum, bool) {
	m := epicNumberRe.FindStringSubmatch(name)
	if m == nil {
		return epicNum{}, false
	}
	n := epicNum{major: -1, minor: -1, patch: -1}
	n.major, _ = strconv.Atoi(m[1])
	if m[2] != "" {
		n.minor, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		n.patch, _ = strconv.Atoi(m[3])
	}
	return n, true
}

// collectEpicNums reads every directory and returns each parsed number found.
// A missing directory is not an error: scanning active/ and completed/ in a
// fresh repo legitimately finds only one of them.
func collectEpicNums(dirs []string) ([]epicNum, error) {
	var out []epicNum
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", d, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if n, ok := parseEpicNum(filepath.Base(e.Name())); ok {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// nextEpicNumber returns the next free epic number across all dirs.
//
// Scanning BOTH active/ and completed/ is the whole point: a number belonging to
// a shipped epic is spent. Reusing it would give two plans the same execution
// position and overwrite history. llm_support_highest cannot express this — it is
// integer-only and non-recursive — which is why this exists separately rather
// than as a flag on that command.
//
// Gaps are never filled. The numbers are execution positions, so a missing 2.0
// may have been retired on purpose; handing it out again would silently insert
// work ahead of everything queued after it.
func nextEpicNumber(dirs []string, parent string) (string, error) {
	nums, err := collectEpicNums(dirs)
	if err != nil {
		return "", err
	}

	if parent == "" {
		highest := 0
		for _, n := range nums {
			if n.major > highest {
				highest = n.major
			}
		}
		return fmt.Sprintf("%d.0", highest+1), nil
	}

	parts := strings.Split(strings.TrimSpace(parent), ".")
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return "", fmt.Errorf("invalid --parent %q: expected N or N.M", parent)
		}
	}

	switch len(parts) {
	case 1: // child of a top-level epic: N -> N.M
		pMajor, _ := strconv.Atoi(parts[0])
		highest := 0
		for _, n := range nums {
			if n.major == pMajor && n.minor > highest {
				highest = n.minor
			}
		}
		return fmt.Sprintf("%d.%d", pMajor, highest+1), nil

	case 2: // child of a child: N.M -> N.M.P
		pMajor, _ := strconv.Atoi(parts[0])
		pMinor, _ := strconv.Atoi(parts[1])
		highest := 0
		for _, n := range nums {
			if n.major == pMajor && n.minor == pMinor && n.patch > highest {
				highest = n.patch
			}
		}
		return fmt.Sprintf("%d.%d.%d", pMajor, pMinor, highest+1), nil

	default:
		return "", fmt.Errorf("invalid --parent %q: epic numbers are at most three levels (N.M.P)", parent)
	}
}

func newEpicNumberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic-number",
		Short: "Pick the next free Epic Plan number",
		Long: `Return the next free Epic Plan number across one or more directories.

Epic numbers are N, N.M, or N.M.P and execute in ascending order, so a number is
an execution POSITION, not just an identity. Two rules follow:

  * Pass every directory that holds epics -- active/ AND completed/. A number
    used by a shipped epic is spent; reusing it would give two plans the same
    position. (llm-support highest cannot do this: it is integer-only and
    non-recursive.)
  * Gaps are never filled. A missing 2.0 may have been retired deliberately.

With --parent, returns the next free CHILD, which is how urgent work is slotted
between existing plans: --parent=3 yields 3.1, which executes before a queued 4.0.

Examples:
  llm-support epic-number --dir .planning/epics/active --dir .planning/epics/completed
  llm-support epic-number --dir .planning/epics/active --dir .planning/epics/completed --parent 3
  llm-support epic-number --dir .planning/epics/active --parent 3.1 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(epicNumberDirs) == 0 {
				return fmt.Errorf("--dir is required (pass it once per directory, e.g. active/ and completed/)")
			}
			next, err := nextEpicNumber(epicNumberDirs, epicNumberParent)
			if err != nil {
				return err
			}
			nums, err := collectEpicNums(epicNumberDirs)
			if err != nil {
				return err
			}
			res := EpicNumberResult{Next: next, Parent: epicNumberParent, InUse: len(nums), Dirs: epicNumberDirs}
			if epicNumberJSON {
				b, err := json.MarshalIndent(res, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "NEXT: %s\nIN_USE: %d\n", res.Next, res.InUse)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&epicNumberDirs, "dir", nil, "Directory holding epic plans (repeat for active/ and completed/)")
	cmd.Flags().StringVar(&epicNumberParent, "parent", "", "Return the next free child of this epic (e.g. 3 or 3.1)")
	cmd.Flags().BoolVar(&epicNumberJSON, "json", false, "Output as JSON")

	return cmd
}

func init() {
	RootCmd.AddCommand(newEpicNumberCmd())
}
