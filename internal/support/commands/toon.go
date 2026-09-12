package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// Streamed, not slurped. Reading the whole file only to inspect its first
	// non-blank line made the command's peak memory the size of its input, and
	// the tabular path then handed those same bytes back through a
	// bytes.Reader — the DecodeFile this replaced streamed straight from the
	// file handle. Only the lines up to the first non-blank one are held; the
	// rest flows to the decoder as it reads.
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	br := bufio.NewReader(f)
	first, consumed, err := peekFirstNonBlankLine(br)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	// The peeked bytes are put back in front of the remainder, rather than
	// seeking, so a non-seekable source such as /dev/stdin still works.
	body := io.MultiReader(bytes.NewReader(consumed), br)

	switch shape {
	case shapeAuto:
		if !toon.IsTabularHeader(first) {
			return emitTOONDocument(cmd, path, body)
		}
	case shapeTabular:
		// Checked BEFORE decoding, so the diagnostic names the real problem
		// rather than whatever the tabular reader happens to trip over first.
		if !toon.IsTabularHeader(first) {
			return fmt.Errorf("%s: not a tabular array: %q does not open one "+
				"(use --shape document, or auto)", path, first)
		}
	case shapeDocument:
		return emitTOONDocument(cmd, path, body)
	default:
		return fmt.Errorf("unknown --shape %q: want %s, %s or %s",
			shape, shapeAuto, shapeTabular, shapeDocument)
	}

	doc, err := toon.Decode(body)
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

func emitTOONDocument(cmd *cobra.Command, path string, r io.Reader) error {
	value, err := toon.DecodeAny(r)
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

// maxPeekBytes bounds the search for the first non-blank line. A file of
// nothing but blank lines would otherwise be read in full by the very code
// added to stop reading files in full.
const maxPeekBytes = 1 << 20

// peekFirstNonBlankLine reads just far enough to find the line the shape
// decision is made on, and returns it alongside every byte consumed so the
// caller can put them back in front of the remainder.
//
// The shape is decided SYNTACTICALLY from that line. Deciding it by attempting
// a tabular decode and falling back when it fails would turn a garbled findings
// payload into a document full of junk, silently and with exit 0 — the failure
// class this whole reader exists to remove.
func peekFirstNonBlankLine(br *bufio.Reader) (string, []byte, error) {
	var consumed []byte
	for len(consumed) < maxPeekBytes {
		line, rerr := br.ReadString('\n')
		consumed = append(consumed, line...)
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) != "" {
			return trimmed, consumed, nil
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return "", consumed, nil
			}
			return "", consumed, rerr
		}
	}
	return "", consumed, nil
}

func init() {
	RootCmd.AddCommand(newTOONCmd())
}
