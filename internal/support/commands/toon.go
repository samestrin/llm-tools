package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/toon"
	"github.com/spf13/cobra"
)

// TOONResult is the JSON shape `toon parse` emits for a tabular array.
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

// TOONDocumentResult is the JSON shape `toon parse` emits for a payload that is
// not a tabular array.
//
// It carries a `shape` discriminator and the TABULAR shape does not, on
// purpose: atcr skills pipe the tabular output straight into other tools, so
// those bytes are a contract and adding a key to them would break it. Absence
// of `shape` means tabular.
//
// Values keep their decoded types — a size stays a number, a flag stays a
// boolean. Nothing consumed this shape before, so there is no string contract
// to preserve here, and typed values are more useful to the caller.
type TOONDocumentResult struct {
	Shape string `json:"shape"`
	Value any    `json:"value"`
}

const (
	shapeAuto     = "auto"
	shapeTabular  = "tabular"
	shapeDocument = "document"
)

func newTOONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toon",
		Short: "Read TOON (Token-Optimized Object Notation) payloads",
		Long: `Read a TOON payload, such as the one atcr emits with
` + "`atcr report --format axi`" + `.

The pipe-delimited findings streams escape a literal '|' to '/' inside free
text, which corrupts exactly the field that matters most — a code fix that
contains a pipe. TOON quotes instead, so the delimiter survives as data.`,
	}
	cmd.AddCommand(newTOONParseCmd())
	return cmd
}

func newTOONParseCmd() *cobra.Command {
	// Held in the closure rather than at package scope so one invocation cannot
	// inherit the previous one's shape.
	var shape string

	cmd := &cobra.Command{
		Use:   "parse <file>",
		Short: "Parse a TOON payload to JSON",
		Long: `Parse a TOON payload and emit it as JSON.

A tabular array emits {name, delimiter, count, declared, fields, meta, rows}.
The declared row count is the TRUE total: atcr paginates, so fewer rows than
declared is legitimate and is reported as a smaller "count".

Any other TOON document emits {shape: "document", value: ...} with its values
typed. --shape tabular refuses to read anything else, which is what a skill
piping atcr output should pass: auto-detection is a convenience, and a silent
downgrade to a shape the caller cannot read is the failure this guards.

Example:
  atcr report --format axi > findings.axi
  llm-support toon parse --shape tabular findings.axi`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTOONParse(cmd, args[0], shape)
		},
	}
	cmd.Flags().StringVar(&shape, "shape", shapeAuto,
		"Payload shape: auto, tabular, or document")
	return cmd
}

func runTOONParse(cmd *cobra.Command, path, shape string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	switch shape {
	case shapeAuto:
		if !firstLineIsTabular(data) {
			return emitTOONDocument(cmd, path, data)
		}
	case shapeTabular:
		// Checked BEFORE decoding, so the diagnostic names the real problem
		// rather than whatever the tabular reader happens to trip over first.
		if !firstLineIsTabular(data) {
			return fmt.Errorf("%s: not a tabular array: %q does not open one "+
				"(use --shape document, or auto)", path, firstNonBlankLine(data))
		}
	case shapeDocument:
		return emitTOONDocument(cmd, path, data)
	default:
		return fmt.Errorf("unknown --shape %q: want %s, %s or %s",
			shape, shapeAuto, shapeTabular, shapeDocument)
	}

	doc, err := toon.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
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
	return emitTOONJSON(cmd, res)
}

func emitTOONDocument(cmd *cobra.Command, path string, data []byte) error {
	value, err := toon.DecodeAny(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return emitTOONJSON(cmd, TOONDocumentResult{Shape: shapeDocument, Value: value})
}

// emitTOONJSON writes compact, not indented. This binary exists to save tokens,
// a findings payload runs to hundreds of rows, and the reader is a model rather
// than a person — indentation would cost real context for no gain. It also
// matches the majority of commands here (45 compact vs 14 indented).
func emitTOONJSON(cmd *cobra.Command, v any) error {
	out, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

// firstNonBlankLine returns the line the shape decision is made on.
func firstNonBlankLine(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimRight(line, "\r")
		}
	}
	return ""
}

// firstLineIsTabular decides the shape SYNTACTICALLY. Deciding it by attempting
// a tabular decode and falling back when it fails would turn a garbled findings
// payload into a document full of junk, silently and with exit 0 — the failure
// class this whole reader exists to remove.
func firstLineIsTabular(data []byte) bool {
	return toon.IsTabularHeader(firstNonBlankLine(data))
}

func init() {
	RootCmd.AddCommand(newTOONCmd())
}
