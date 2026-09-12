package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	epicNumberDirs   []string
	epicNumberParent string
	epicNumberJSON   bool
)

// epicNumberRe matches a leading dot-separated epic number of ANY depth,
// followed by a separator. Anything else in the directory (README.md, notes,
// dotfiles) is not an epic and is ignored rather than treated as an error.
//
// Depth is deliberately uncapped. A three-level cap did not reject a deeper
// name, it failed to MATCH one -- so a 35.16.6.4 plan became invisible to the
// scan and the number was free to be issued to a second plan. A number the tool
// cannot see is worse than one it refuses.
var epicNumberRe = regexp.MustCompile(`^(\d+(?:\.\d+)*)[-_]`)

// EpicNumberResult is the payload returned to callers.
type EpicNumberResult struct {
	Next   string   `json:"next"`
	Parent string   `json:"parent,omitempty"`
	InUse  int      `json:"in_use"`
	Dirs   []string `json:"dirs"`
}

// epicNum is a parsed epic number, one element per level: 3.1.2 is {3, 1, 2}.
// Depth is carried rather than fixed, so 3 and 3.0 stay distinguishable from
// 3.1 when deciding what a "child" means.
type epicNum []int

// parseEpicNum reads the leading number off an epic filename.
func parseEpicNum(name string) (epicNum, bool) {
	m := epicNumberRe.FindStringSubmatch(name)
	if m == nil {
		return nil, false
	}
	parts := strings.Split(m[1], ".")
	n := make(epicNum, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		n = append(n, v)
	}
	return n, true
}

// childLevel reports the value this number contributes at the level directly
// below prefix, and whether it contributes one at all.
//
// Matching is by PREFIX, not by equal depth: 35.16.6.4 contributes 6 below
// 35.16 exactly as a bare 35.16.6 would. Requiring equal depth would let
// nextEpicNumber hand out 35.16.6 while 35.16.6.4 sits in the directory,
// inserting a plan ahead of everything queued under it.
func (n epicNum) childLevel(prefix []int) (int, bool) {
	if len(n) <= len(prefix) {
		return 0, false
	}
	for i, p := range prefix {
		if n[i] != p {
			return 0, false
		}
	}
	return n[len(prefix)], true
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
			if len(n) > 0 && n[0] > highest {
				highest = n[0]
			}
		}
		return fmt.Sprintf("%d.0", highest+1), nil
	}

	prefix, err := parseParent(parent)
	if err != nil {
		return "", err
	}

	highest := 0
	for _, n := range nums {
		if v, ok := n.childLevel(prefix); ok && v > highest {
			highest = v
		}
	}
	return fmt.Sprintf("%s.%d", strings.TrimSpace(parent), highest+1), nil
}

// parseParent validates a --parent value into its levels. Depth is uncapped;
// only the components must be numeric.
func parseParent(parent string) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(parent), ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid --parent %q: every level must be a number (e.g. 3, 3.1, 35.16.6)", parent)
		}
		out = append(out, v)
	}
	return out, nil
}

func newEpicNumberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic-number",
		Short: "Pick the next free Epic Plan number",
		Long: `Return the next free Epic Plan number across one or more directories.

Epic numbers are dot-separated to any depth (N, N.M, N.M.P, N.M.P.Q ...) and
execute in ascending order, so a number is an execution POSITION, not just an
identity. Two rules follow:

  * Pass every directory that holds epics -- active/ AND completed/. A number
    used by a shipped epic is spent; reusing it would give two plans the same
    position. (llm-support highest cannot do this: it is integer-only and
    non-recursive.)
  * Gaps are never filled. A missing 2.0 may have been retired deliberately.

With --parent, returns the next free CHILD, which is how urgent work is slotted
between existing plans: --parent=3 yields 3.1, which executes before a queued 4.0.
Parent depth is uncapped, so --parent=35.16.6 yields 35.16.6.N -- which runs next,
where a shallower 35.16.N would land at the back of that family's queue.

Examples:
  llm-support epic-number --dir .planning/epics/active --dir .planning/epics/completed
  llm-support epic-number --dir .planning/epics/active --dir .planning/epics/completed --parent 3
  llm-support epic-number --dir .planning/epics/active --parent 3.1 --json
  llm-support epic-number --dir .planning/epics/active --parent 35.16.6`,
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
			if GlobalAXIOutput {
				return output.EncodeAXI(cmd.OutOrStdout(), res)
			}
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
	cmd.Flags().StringVar(&epicNumberParent, "parent", "", "Return the next free child of this epic, any depth (e.g. 3, 3.1, 35.16.6)")
	cmd.Flags().BoolVar(&epicNumberJSON, "json", false, "Output as JSON")

	return cmd
}

func init() {
	RootCmd.AddCommand(newEpicNumberCmd())
}
