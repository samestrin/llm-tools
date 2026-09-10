package commands

import (
	"encoding/json"
	"fmt"

	"github.com/samestrin/llm-tools/internal/support/toon"
	"github.com/spf13/cobra"
)

// TOONResult is the JSON shape `toon parse` emits.
//
// Rows are maps rather than positional arrays because every consumer reads by
// field name, and `fields` is kept alongside so the declared ORDER survives —
// a map has none, and re-encoding needs it.
type TOONResult struct {
	Name      string   `json:"name"`
	Delimiter string   `json:"delimiter"`
	Count     int      `json:"count"`
	Declared  int      `json:"declared"`
	Fields    []string `json:"fields"`

	// Meta carries sibling `key: value` lines that follow the array. atcr emits
	// `truncated: <bool>` there, and a consumer needs it: when truncated,
	// `declared` is the TRUE total and `count` is what physically arrived.
	Meta map[string]string   `json:"meta"`
	Rows []map[string]string `json:"rows"`
}

func newTOONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toon",
		Short: "Read TOON (Token-Optimized Object Notation) payloads",
		Long: `Read a TOON tabular array, such as the payload atcr emits with
` + "`atcr report --format axi`" + `.

The pipe-delimited findings streams escape a literal '|' to '/' inside free
text, which corrupts exactly the field that matters most — a code fix that
contains a pipe. TOON quotes instead, so the delimiter survives as data.`,
	}
	cmd.AddCommand(newTOONParseCmd())
	return cmd
}

func newTOONParseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse <file>",
		Short: "Parse a TOON tabular array to JSON",
		Long: `Parse a TOON tabular array and emit it as JSON.

The declared row count is a gate: a payload whose rows disagree with its header
is an error, not a short list that looks complete.

Example:
  atcr report --format axi > findings.axi
  llm-support toon parse findings.axi`,
		Args: cobra.ExactArgs(1),
		RunE: runTOONParse,
	}
}

func runTOONParse(cmd *cobra.Command, args []string) error {
	doc, err := toon.DecodeFile(args[0])
	if err != nil {
		return err
	}
	res := TOONResult{
		Name:      doc.Name,
		Delimiter: string(doc.Delimiter),
		Count:     len(doc.Rows),
		Declared:  doc.Declared,
		Fields:    doc.Fields,
		Meta:      doc.Meta,
		Rows:      doc.Rows,
	}
	// Never null: a zero-findings review is a well-formed payload, and a consumer
	// iterating `rows` should get an empty list rather than a nil it must guard.
	if res.Fields == nil {
		res.Fields = []string{}
	}
	if res.Rows == nil {
		res.Rows = []map[string]string{}
	}
	if res.Meta == nil {
		res.Meta = map[string]string{}
	}
	// Compact, not indented. This binary exists to save tokens, a findings
	// payload runs to hundreds of rows, and the reader is a model rather than a
	// person — indentation would cost real context for no gain. It also matches
	// the majority of commands here (45 compact vs 14 indented).
	out, err := json.Marshal(res)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

func init() {
	RootCmd.AddCommand(newTOONCmd())
}
