// Package toon reads TOON (Token-Optimized Object Notation) payloads.
//
// It exists because atcr already SPEAKS this format and nothing here could
// listen. `atcr report --format axi` emits a versioned, token-dense payload;
// the consumers were reading atcr's pipe-delimited form instead and escaping
// `|` to `/` by hand inside free text, which loses data in exactly the fields
// that matter — a code fix containing a pipe.
//
// The codec is github.com/toon-format/toon-go. This package is the part
// toon-go does not cover: it reads the header, isolates one array, and hands
// the codec a document it can accept.
//
//	findings[2|]{severity|"file:line"|problem|fix}:
//	  CRITICAL|"auth.go:42"|token never expires|check expiry
//	  LOW|"util.go:7"|unused var|""
//
// Header: NAME [ COUNT DELIM ] { FIELD... } ':'  — then up to COUNT rows,
// each indented, values separated by DELIM.
//
// Three things about that header are easy to get wrong, and all three are
// load-bearing:
//
//   - **The delimiter is declared, not assumed.** It is the character between
//     the count and `]`. A reader that hardcodes `|` mis-splits every other
//     delimiter without complaining.
//   - **Header fields are quoted by the same rules as values.** `"file:line"`
//     is quoted because a colon forces quoting, so the field NAME is
//     `file:line` — a reader that keeps the quotes keys every consumer wrong.
//   - **Fewer rows than declared is legitimate.** atcr paginates and keeps the
//     true total in the header. More rows than declared is not.
//
// # Why the array is isolated rather than forwarded
//
// toon-go decodes a whole document in one strict pass, and three of its rules
// make that unusable here:
//
//   - an explicitly written comma delimiter is rejected outright, though
//     llm-tools has shipped payloads carrying one;
//   - a trailing block (atcr's paginated `help[...]`) that declares more rows
//     than it carries fails the whole document, taking the findings array with
//     it;
//   - a row count that disagrees with the physical rows is an error in either
//     direction, which cannot read atcr's paginated output at all.
//
// So the header is read here, the rows belonging to THIS array are collected
// here, and toon-go receives a synthesized document carrying the physical row
// count and a delimiter it accepts. Strict mode stays on, which keeps its
// row-width gate. Everything after the array — sibling scalars, trailing
// blocks — never reaches it.
package toon

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	gotoon "github.com/toon-format/toon-go"
)

// Document is one decoded tabular array.
type Document struct {
	Name      string              // the array name, e.g. "findings"
	Fields    []string            // declared field names, in order, unquoted
	Delimiter rune                // read from the header, never assumed
	Rows      []map[string]string // one map per row, keyed by field name

	// Declared is the row count the HEADER states. It is not always len(Rows):
	// atcr's paginated payload caps the emitted rows while keeping the true
	// total in the header, and says so with a `truncated` sibling.
	Declared int

	// Meta holds `key: value` sibling lines that follow the array at column 0 —
	// `truncated: false` is the one atcr emits today.
	Meta map[string]string
}

// Fields is order-bearing and Rows is not, so a caller that needs positional
// values rebuilds them from the two. That is lossless because duplicate field
// names are rejected at parse time.

// maxLine bounds one physical line. Findings carry free text; bufio.Scanner's
// 64KB default is too small for a long PROBLEM or FIX, and the failure mode is
// a silently truncated line.
const maxLine = 8 * 1024 * 1024

// MaxDocumentBytes bounds a whole document read by DecodeAny.
//
// toon-go's decoder takes a []byte, so the document has to be resident — but
// "resident" is not "unbounded". DecodeAny accepts any io.Reader, including one
// whose size cannot be checked first, and an uncapped read there is a way to
// exhaust memory before a single byte is parsed. 64 MiB is far above any real
// findings payload (hundreds of rows of KB-scale text) and far below trouble.
const MaxDocumentBytes = 64 * 1024 * 1024

// Decode reads one TOON tabular array from r.
func Decode(r io.Reader) (*Document, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)

	var (
		headerLine string
		headerAt   int
		line       int
	)
	for sc.Scan() {
		line++
		if strings.TrimSpace(sc.Text()) != "" {
			headerLine, headerAt = sc.Text(), line
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("toon: line %d: %w", line+1, err)
	}
	if headerLine == "" {
		return nil, fmt.Errorf("toon: no header found: expected a line like " +
			`name[N|]{field|field}:`)
	}

	h, err := parseHeader(headerLine, headerAt)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Name:      h.name,
		Fields:    h.fields,
		Delimiter: h.delimiter,
		Declared:  h.declared,
		Meta:      map[string]string{},
	}

	// INDENTATION is the discriminator, and it has to be. `atcr report --axi`
	// appends `truncated: <bool>` at column 0 after the rows, and Epic 42.0
	// appends a whole `help[]` block. A reader that consumes every line to EOF
	// swallows both and reports a column-count mismatch on real CLI output —
	// which is what this package did until it was run against the binary rather
	// than against the encoder's golden fixture.
	var rows []string
	for sc.Scan() {
		line++
		raw := sc.Text()
		if strings.TrimSpace(raw) == "" {
			// Today's reader skips a blank line between rows. Strict toon-go
			// rejects one, so it is dropped here rather than forwarded.
			continue
		}
		if raw[0] != ' ' && raw[0] != '\t' {
			// End of this array's rows. A scalar `key: value` is sibling
			// metadata worth keeping; anything else is a following block and
			// not ours.
			collectSiblings(sc, raw, doc.Meta)
			break
		}
		// Re-indented to exactly two spaces. toon-go requires the indent to be
		// a multiple of two; the old reader accepted any leading whitespace and
		// trimmed it, so normalising here preserves that tolerance instead of
		// turning it into an error.
		rows = append(rows, "  "+strings.TrimLeft(raw, " \t"))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("toon: line %d: %w", line+1, err)
	}

	// The gate, in ONE direction only. More rows than the header declares is a
	// disagreement no contract allows. FEWER is legitimate and load-bearing:
	// atcr's paginated payload is "intentionally NOT length-round-trippable —
	// the header declares N (the true total) while fewer than N rows are
	// physically present", and it says so with `truncated`. A hard equality
	// gate cannot read that output at all, so the caller gets both numbers and
	// decides.
	if len(rows) > h.declared {
		return nil, fmt.Errorf("toon: header declares %d row(s) but the payload carries %d",
			h.declared, len(rows))
	}

	decoded, err := gotoon.DecodeString(synthesize(h, rows, headerAt))
	if err != nil {
		return nil, fmt.Errorf("toon: %w", err)
	}

	values, err := arrayOf(decoded, h.name)
	if err != nil {
		return nil, err
	}
	for i, v := range values {
		row, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("toon: row %d is %T, not a record", i+1, v)
		}
		out := make(map[string]string, len(row))
		for k, raw := range row {
			s, err := projectValue(raw)
			if err != nil {
				return nil, fmt.Errorf("toon: row %d, field %q: %w", i+1, k, err)
			}
			out[k] = s
		}
		doc.Rows = append(doc.Rows, out)
	}
	return doc, nil
}

// synthesize builds the canonical single-array document handed to toon-go.
//
// The header carries the PHYSICAL row count, so toon-go's length check can only
// fire on a disagreement this package has already decided to allow. The leading
// blank lines put the header back on its original line number, because toon-go
// reports errors by line and its error type is unexported — padding is the only
// way to keep those numbers pointing at the caller's file.
//
// KNOWN LIMITATION, asserted rather than claimed fixed: a blank line dropped
// from BETWEEN rows shifts every reported line after it by one. It cannot be
// closed by forwarding the blank, because strict toon-go rejects a blank line
// inside a tabular array outright while the old reader skipped them — so
// forwarding would turn a payload that parses today into an error.
// TestABlankLineBetweenRowsShiftsReportedLineNumbers pins the real behaviour.
func synthesize(h *header, rows []string, headerAt int) string {
	var b strings.Builder
	for i := 1; i < headerAt; i++ {
		b.WriteByte('\n')
	}
	b.WriteString(h.nameText)
	b.WriteByte('[')
	b.WriteString(strconv.Itoa(len(rows)))
	b.WriteString(delimiterSuffix(h.delimiter))
	b.WriteByte(']')
	if h.hasFields {
		b.WriteByte('{')
		b.WriteString(h.fieldsText)
		b.WriteByte('}')
	}
	b.WriteString(":\n")
	for _, r := range rows {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	return b.String()
}

// arrayOf pulls this array out of the decoded document. A named array decodes
// as one key of a map; an unnamed one (`[2]{id}:`) decodes as the document
// itself.
func arrayOf(decoded any, name string) ([]any, error) {
	if name == "" {
		if arr, ok := decoded.([]any); ok {
			return arr, nil
		}
		return nil, fmt.Errorf("toon: document is %T, not an array", decoded)
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("toon: document is %T, not a named array", decoded)
	}
	v, ok := m[name]
	if !ok {
		return nil, fmt.Errorf("toon: array %q is missing from the decoded document", name)
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("toon: %q is %T, not an array", name, v)
	}
	return arr, nil
}

// projectValue renders one decoded value as the string every caller expects.
//
// toon-go types its output: a number is float64, a keyword is bool, and the
// literal `null` is nil (an EMPTY token decodes to "", so the two stay
// distinct). Document.Rows is map[string]string and three callers read it that
// way — group_td asserts item["EST_MINUTES"] == "15" as a string, and reads
// every cell with a discarded comma-ok assertion, so a typed value there
// renders as an empty table cell with no error and exit 0.
//
// An unquoted number loses its exact digits (0.50 becomes 0.5), because the raw
// token is unreachable once toon-go has parsed it. Quoting is the escape hatch
// and atcr already quotes what matters.
func projectValue(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case bool:
		return strconv.FormatBool(t), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case nil:
		return "null", nil
	default:
		return "", fmt.Errorf("value of type %T has no tabular form", v)
	}
}

// collectSiblings reads the `key: value` scalars that follow an array at column
// 0, beginning with first.
//
// Every one of them, not just the first. Document.Meta is documented as holding
// sibling LINES, and stopping after one dropped each later scalar in silence —
// atcr emits a single `truncated` today, so nothing had lost data yet, but the
// contract said otherwise. A line that is not a scalar key/value — a following
// block header, say — ends collection and is left alone, as does a return to
// indented content.
func collectSiblings(sc *bufio.Scanner, first string, meta map[string]string) {
	raw := first
	for {
		k, v, ok := siblingKV(raw)
		if !ok {
			return
		}
		meta[k] = v
		for {
			if !sc.Scan() {
				return
			}
			raw = sc.Text()
			if strings.TrimSpace(raw) != "" {
				break
			}
		}
		if raw[0] == ' ' || raw[0] == '\t' {
			return
		}
	}
}

// siblingKV reads a `key: value` metadata line that follows an array at column
// 0. atcr emits exactly one today, `truncated: <bool>`, deliberately as a bare
// TOON scalar rather than an array row. A line that is not scalar `key: value`
// — a following block header, say — is not metadata and is left alone.
func siblingKV(raw string) (string, string, bool) {
	i := strings.IndexByte(raw, ':')
	if i <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(raw[:i])
	val := strings.TrimSpace(raw[i+1:])
	if key == "" || val == "" || strings.ContainsAny(key, " \t[]{}") {
		return "", "", false
	}
	return key, val, true
}

// DecodeFile reads one TOON tabular array from a file, naming it on error.
func DecodeFile(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	doc, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// DecodeAny reads a TOON document of any shape, preserving toon-go's types.
//
// Decode is tabular-only by contract and rejects anything else, which is
// correct for the findings payloads it was written for and useless for the
// rest. `llm-filesystem-axi get-file-info` emits a flat object of scalars with
// no array anywhere, and `list-directory` emits an array with a `total`
// sibling — the first would not decode at all and the second silently dropped
// the total.
//
// Types are preserved rather than projected to strings: this is a new entry
// point with no existing consumers, and a size that stays a number is more
// useful than one that becomes text.
func DecodeAny(r io.Reader) (any, error) {
	// One byte past the ceiling is read deliberately, so a document exactly at
	// the limit still decodes and anything larger is reported rather than
	// silently truncated into a payload that parses.
	data, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxDocumentBytes {
		return nil, fmt.Errorf("toon: document is too large: over %d bytes", MaxDocumentBytes)
	}
	decoded, err := gotoon.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("toon: %w", err)
	}
	// toon-go reports an empty document as a non-nil, EMPTY container rather
	// than as an error. Probed, not assumed: "" and "\n" decode to
	// map[string]any{}, and `[0]:` decodes to a top-level []any{} — so neither
	// a `decoded == nil` guard nor a map-only guard ever fires for it. An empty
	// success is the trap go-axi's Check exists to catch, and it must not reach
	// a caller as a valid document.
	switch v := decoded.(type) {
	case nil:
		return nil, fmt.Errorf("toon: empty document")
	case map[string]any:
		if len(v) == 0 {
			return nil, fmt.Errorf("toon: empty document")
		}
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("toon: empty document")
		}
	}
	return decoded, nil
}

// DecodeAnyFile reads a TOON document of any shape from a file.
func DecodeAnyFile(path string) (any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	doc, err := DecodeAny(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}
