package commands

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var (
	tdValidatePath string
	tdValidateRoot string
	tdValidateMode string
	tdValidateJSON bool
	tdValidateMin  bool
)

// TDValidateItem holds the validation result for a single TD row.
type TDValidateItem struct {
	Group       string `json:"group"`
	Checkbox    string `json:"checkbox"`
	Severity    string `json:"severity"`
	FileLine    string `json:"file_line"`
	FilePath    string `json:"file_path"`
	Symbol      string `json:"symbol"`
	FileExists  bool   `json:"file_exists"`
	SymbolFound *bool  `json:"symbol_found"` // null=no symbol; true=found; false=not found
	Status      string `json:"status"`       // valid|file_missing|symbol_not_found|no_file
	Section     string `json:"section"`
}

// TDValidateSummary holds aggregate counts across all validated rows.
type TDValidateSummary struct {
	Total          int `json:"total"`
	Valid          int `json:"valid"`
	FileMissing    int `json:"file_missing"`
	SymbolNotFound int `json:"symbol_not_found"`
	NoFile         int `json:"no_file"`
	OpenChecked    int `json:"open_checked"`
	DeferredChecked int `json:"deferred_checked"`
}

// TDValidateResult is the full JSON payload returned by td-validate.
type TDValidateResult struct {
	Items   []TDValidateItem  `json:"items"`
	Summary TDValidateSummary `json:"summary"`
}

func newTDValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-validate",
		Short: "Verify cited files and symbols in open TD items still exist",
		RunE:  runTDValidate,
	}
	cmd.Flags().StringVar(&tdValidatePath, "path", "", "Path to TD README (required)")
	cmd.Flags().StringVar(&tdValidateRoot, "root", ".", "Repo root for resolving relative file paths")
	cmd.Flags().StringVar(&tdValidateMode, "mode", "open", "Rows to check: open or all")
	cmd.Flags().BoolVar(&tdValidateJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdValidateMin, "min", false, "Minimal output")
	cmd.MarkFlagRequired("path")
	return cmd
}

func runTDValidate(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("not implemented")
}

// parseFileLine extracts a file path and optional symbol from a TD FileLine cell.
// Returns (filePath, symbol, ok); ok=false means the field is empty or unparseable.
func parseFileLine(raw string) (filePath, symbol string, ok bool) {
	return "", "", false
}

func printTDValidateText(w io.Writer, data interface{}) {
	r := data.(*TDValidateResult)
	fmt.Fprintf(w, "td_validate: %d item(s) checked — %d valid, %d file_missing, %d symbol_not_found\n",
		r.Summary.Total, r.Summary.Valid, r.Summary.FileMissing, r.Summary.SymbolNotFound)
}

func init() {
	RootCmd.AddCommand(newTDValidateCmd())
}
