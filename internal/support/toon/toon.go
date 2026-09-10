// Package toon reads TOON (Token-Optimized Object Notation) tabular arrays.
//
// It exists because atcr already SPEAKS this format and nothing here could
// listen. `atcr report --format axi` emits a versioned, token-dense payload;
// the consumers were reading atcr's pipe-delimited form instead and escaping
// `|` to `/` by hand inside free text, which loses data in exactly the fields
// that matter — a code fix containing a pipe.
//
// The tabular array is the only shape implemented, because it is the only shape
// the findings contract uses:
//
//	findings[2|]{severity|"file:line"|problem|fix}:
//	  CRITICAL|"auth.go:42"|token never expires|check expiry
//	  LOW|"util.go:7"|unused var|""
//
// Header: NAME [ COUNT DELIM ] { FIELD... } ':'  — then exactly COUNT rows,
// each indented, values separated by DELIM.
//
// Two things about that header are easy to get wrong, and both are load-bearing:
//
//   - **The delimiter is declared, not assumed.** It is the character between
//     the count and `]`. A reader that hardcodes `|` mis-splits every other
//     delimiter without complaining.
//   - **Header fields are quoted by the same rules as values.** `"file:line"`
//     is quoted because a colon forces quoting, so the field NAME is
//     `file:line` — a reader that keeps the quotes keys every consumer wrong.
//
// The declared count is a gate rather than a hint. A payload whose row count
// disagrees with its header is truncated or corrupt, and a reader that accepts
// it hands the caller a short list that looks complete.
package toon

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Document is one decoded tabular array.
type Document struct {
	Name      string              // the array name, e.g. "findings"
	Fields    []string            // declared field names, in order, unquoted
	Delimiter rune                // read from the header, never assumed
	Rows      []map[string]string // one map per row, keyed by field name
}

// Fields is order-bearing and Rows is not, so a caller that needs positional
// values rebuilds them from the two. That is lossless because duplicate field
// names are rejected at parse time.

// Decode reads one TOON tabular array from r.
func Decode(r io.Reader) (*Document, error) {
	sc := bufio.NewScanner(r)
	// Findings carry free text; the 64KB default token is too small for a long
	// PROBLEM or FIX, and the failure mode is a silently truncated line.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var header string
	line := 0
	for sc.Scan() {
		line++
		if strings.TrimSpace(sc.Text()) != "" {
			header = sc.Text()
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if header == "" {
		return nil, fmt.Errorf("toon: no header found: expected a line like " +
			`name[N|]{field|field}:`)
	}

	doc, count, err := parseHeader(header, line)
	if err != nil {
		return nil, err
	}

	for sc.Scan() {
		line++
		raw := sc.Text()
		if strings.TrimSpace(raw) == "" {
			continue
		}
		values, err := splitRow(strings.TrimLeft(raw, " \t"), doc.Delimiter, line)
		if err != nil {
			return nil, err
		}
		if len(values) != len(doc.Fields) {
			return nil, fmt.Errorf("toon: line %d: row has %d value(s), header declares %d field(s)",
				line, len(values), len(doc.Fields))
		}
		row := make(map[string]string, len(values))
		for i, f := range doc.Fields {
			row[f] = values[i]
		}
		doc.Rows = append(doc.Rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// The gate. A truncated payload is indistinguishable from a complete one
	// without it, and the caller would act on a short list that looks whole.
	if len(doc.Rows) != count {
		return nil, fmt.Errorf("toon: header declares %d row(s) but the payload carries %d",
			count, len(doc.Rows))
	}
	return doc, nil
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

// parseHeader reads `NAME[COUNT DELIM]{FIELDS}:` or the empty `NAME[0]:` form.
func parseHeader(header string, line int) (*Document, int, error) {
	open := strings.IndexByte(header, '[')
	if open < 0 {
		return nil, 0, fmt.Errorf("toon: line %d: header has no `[`: %q", line, header)
	}
	shut := strings.IndexByte(header[open:], ']')
	if shut < 0 {
		return nil, 0, fmt.Errorf("toon: line %d: header has no `]`: %q", line, header)
	}
	shut += open

	doc := &Document{Name: header[:open], Delimiter: ','}
	spec := header[open+1 : shut]

	// The delimiter is the trailing character when one is present. `[0]` carries
	// no delimiter because an empty array has no row to split.
	digits := spec
	if n := len(spec); n > 0 {
		if last := rune(spec[n-1]); last < '0' || last > '9' {
			doc.Delimiter, digits = last, spec[:n-1]
		}
	}
	count, err := strconv.Atoi(digits)
	if err != nil {
		return nil, 0, fmt.Errorf("toon: line %d: %q is not a row count in %q", line, digits, header)
	}

	rest := header[shut+1:]
	if !strings.HasSuffix(strings.TrimRight(rest, " \t"), ":") {
		return nil, 0, fmt.Errorf("toon: line %d: header does not end in `:`: %q", line, header)
	}
	rest = strings.TrimSuffix(strings.TrimRight(rest, " \t"), ":")

	// The empty form has no field list at all, so a zero-row payload is
	// well-formed rather than an error (atcr emits `findings[0]:` for a clean
	// review).
	if rest == "" {
		if count != 0 {
			return nil, 0, fmt.Errorf("toon: line %d: header declares %d row(s) but no fields", line, count)
		}
		return doc, 0, nil
	}
	if !strings.HasPrefix(rest, "{") || !strings.HasSuffix(rest, "}") {
		return nil, 0, fmt.Errorf("toon: line %d: field list is not wrapped in `{}`: %q", line, header)
	}
	fields, err := splitRow(rest[1:len(rest)-1], doc.Delimiter, line)
	if err != nil {
		return nil, 0, err
	}
	// A field name has to be usable as a map key, and unique. An empty name is
	// one no consumer can ask for; a repeat collapses two columns into one with
	// last-value-wins and loses a column in silence — which is the failure mode
	// this reader exists to remove.
	seen := make(map[string]bool, len(fields))
	for i, f := range fields {
		if f == "" {
			return nil, 0, fmt.Errorf("toon: line %d: field %d has an empty name", line, i+1)
		}
		if seen[f] {
			return nil, 0, fmt.Errorf("toon: line %d: field %q is declared twice; "+
				"two columns would collapse into one", line, f)
		}
		seen[f] = true
	}
	doc.Fields = fields
	return doc, count, nil
}

// splitRow splits one delimited line, honouring quoted values, and unescapes
// each. A delimiter inside quotes is data — that is the whole reason the format
// quotes at all, and the reason the pipe-delimited predecessor lost fixes.
func splitRow(s string, delim rune, line int) ([]string, error) {
	var (
		out    []string
		cur    strings.Builder
		quoted bool
		esc    bool
	)
	flush := func() {
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, r := range s {
		switch {
		case esc:
			switch r {
			case 'n':
				cur.WriteByte('\n')
			case 'r':
				cur.WriteByte('\r')
			case 't':
				cur.WriteByte('\t')
			case '\\', '"':
				cur.WriteRune(r)
			default:
				// Not a TOON escape. Keep both characters rather than guess:
				// silently dropping a backslash would corrupt a Windows path.
				cur.WriteByte('\\')
				cur.WriteRune(r)
			}
			esc = false
		case quoted && r == '\\':
			esc = true
		case r == '"' && !quoted && cur.Len() == 0:
			quoted = true
		case r == '"' && quoted:
			quoted = false
		case r == delim && !quoted:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if quoted {
		return nil, fmt.Errorf("toon: line %d: unterminated quote", line)
	}
	if esc {
		return nil, fmt.Errorf("toon: line %d: line ends in a dangling escape", line)
	}
	flush()
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("toon: line %d: not valid UTF-8", line)
	}
	return out, nil
}
