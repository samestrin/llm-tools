package toon

import (
	"fmt"
	"strconv"
	"strings"
)

// header is a decoded tabular-array header line.
//
// This is the part toon-go cannot give back. Its parsedHeader type is
// unexported and Decode returns map[string]any, so the declared field ORDER,
// the delimiter and the declared row count are all unreachable from the public
// API. Reading the header here is structurally required, not a stylistic
// preference.
type header struct {
	// name is the array name with any quoting removed, which is how toon-go
	// keys it in the decoded document. A space or a colon forces quoting on an
	// array name by exactly the rules that quote a field name, so a reader that
	// keeps the quotes cannot find its own array.
	name string

	// nameText is the name exactly as written, quotes included. The synthesized
	// document reuses it verbatim, because stripping the quotes there would
	// produce a header toon-go rejects as an invalid unquoted key.
	nameText string

	// fields are the declared names, in order, unquoted.
	fields []string

	// fieldsText is the field list exactly as written, braces excluded. The
	// synthesized document reuses it verbatim so toon-go applies its own
	// unquoting rules to the keys it builds rows with.
	fieldsText string

	// hasFields distinguishes `findings[0]:` (no field list at all) from
	// `findings[1|]{}:` (one field with an empty name, which is an error).
	hasFields bool

	delimiter rune
	declared  int
}

// parseHeader reads `NAME[COUNT DELIM]{FIELDS}:` or the empty `NAME[0]:` form.
func parseHeader(line string, lineNo int) (*header, error) {
	open := strings.IndexByte(line, '[')
	if open < 0 {
		return nil, fmt.Errorf("toon: line %d: header has no `[`: %q", lineNo, line)
	}
	shut := strings.IndexByte(line[open:], ']')
	if shut < 0 {
		return nil, fmt.Errorf("toon: line %d: header has no `]`: %q", lineNo, line)
	}
	shut += open

	nameText := line[:open]
	h := &header{
		name:      unquoteFieldName(nameText),
		nameText:  nameText,
		delimiter: ',',
	}
	spec := line[open+1 : shut]

	// A leading '#' is the optional length marker (`[#2]`). toon-go emits it
	// under WithLengthMarkers and it carries no extra meaning here — the count
	// that follows is the same count.
	spec = strings.TrimPrefix(spec, "#")

	// The delimiter is the trailing character when one is present. `[0]` carries
	// no delimiter because an empty array has no row to split.
	digits := spec
	if n := len(spec); n > 0 {
		if last := rune(spec[n-1]); last < '0' || last > '9' {
			h.delimiter, digits = last, spec[:n-1]
		}
	}
	count, err := strconv.Atoi(digits)
	if err != nil {
		return nil, fmt.Errorf("toon: line %d: %q is not a row count in %q", lineNo, digits, line)
	}
	h.declared = count

	// TOON defines exactly three delimiters. The old reader accepted any
	// character, which meant a payload could declare something no encoder emits
	// and no other reader accepts.
	switch h.delimiter {
	case ',', '|', '\t':
	default:
		return nil, fmt.Errorf("toon: line %d: %q is not a TOON delimiter "+
			"(comma, pipe or tab): %q", lineNo, string(h.delimiter), line)
	}

	rest := line[shut+1:]
	if !strings.HasSuffix(strings.TrimRight(rest, " \t"), ":") {
		return nil, fmt.Errorf("toon: line %d: header does not end in `:`: %q", lineNo, line)
	}
	rest = strings.TrimSuffix(strings.TrimRight(rest, " \t"), ":")

	// The empty form has no field list at all, so a zero-row payload is
	// well-formed rather than an error (atcr emits `findings[0]:` for a clean
	// review).
	if rest == "" {
		if count != 0 {
			return nil, fmt.Errorf("toon: line %d: header declares %d row(s) but no fields", lineNo, count)
		}
		h.fields = []string{}
		return h, nil
	}
	if !strings.HasPrefix(rest, "{") || !strings.HasSuffix(rest, "}") {
		return nil, fmt.Errorf("toon: line %d: field list is not wrapped in `{}`: %q", lineNo, line)
	}
	h.hasFields = true
	h.fieldsText = rest[1 : len(rest)-1]

	for _, raw := range splitFieldList(h.fieldsText, h.delimiter) {
		h.fields = append(h.fields, unquoteFieldName(raw))
	}

	// A field name has to be usable as a map key, and unique. An empty name is
	// one no consumer can ask for; a repeat collapses two columns into one with
	// last-value-wins and loses a column in silence — which is the failure mode
	// this reader exists to remove. toon-go accepts a duplicate and keeps one
	// column, so this guard cannot be delegated.
	seen := make(map[string]bool, len(h.fields))
	for i, f := range h.fields {
		if f == "" {
			return nil, fmt.Errorf("toon: line %d: field %d has an empty name", lineNo, i+1)
		}
		if seen[f] {
			return nil, fmt.Errorf("toon: line %d: field %q is declared twice; "+
				"two columns would collapse into one", lineNo, f)
		}
		seen[f] = true
	}
	return h, nil
}

// IsTabularHeader reports whether line has the SHAPE of a TOON tabular array
// header: a name, a bracketed count, and a trailing colon.
//
// It is the dispatch point between Decode and DecodeAny, and it deliberately
// does NOT validate. Asking parseHeader instead — "does this header parse" —
// makes a BROKEN tabular header indistinguishable from a line that was never a
// tabular header, and the caller then reroutes it to the document reader.
// Observed, through the binary rather than the package:
//
//	$ llm-support toon parse f.axi          # f[1|]{a|b|a}: / 1|2|3
//	{"shape":"document","value":{"f":[{"a":3,"b":2}]}}          exit 0
//
// A duplicate column is an error at the package level and there is a test
// proving it, but the payload never reached that error. toon-go kept one of
// the two `a` columns, last value wins, the 1 was gone, and the command
// reported success. Dropping a column in silence is the precise failure this
// reader exists to remove, so a line that LOOKS tabular stays on the tabular
// path and gets to report its real problem.
//
// The shape test cannot swallow a document: an inline array (`tags[2]: x,y`)
// does not end in a colon, and a scalar or nested key has no bracket at all.
//
// It also replaces the private header regexp parse_stream carries. Two
// definitions of "is this TOON" in one binary drift apart, and that one is
// already stricter about the array name than this package has ever been.
func IsTabularHeader(line string) bool {
	open := strings.IndexByte(line, '[')
	if open < 0 {
		return false
	}
	shut := strings.IndexByte(line[open:], ']')
	if shut < 0 {
		return false
	}
	return strings.HasSuffix(strings.TrimRight(line[open+shut+1:], " \t"), ":")
}

// splitFieldList splits a declared field list on the delimiter, honouring
// quotes. It is deliberately much smaller than the value splitter it replaces:
// it does not unescape, because a field NAME is an identifier rather than free
// text. The escaping risk that justified the old splitter lived entirely in
// values — a pipe inside a finding, a Windows path, an embedded newline — and
// toon-go handles those now.
func splitFieldList(s string, delim rune) []string {
	var (
		out    []string
		cur    strings.Builder
		quoted bool
		esc    bool
	)
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case quoted && r == '\\':
			cur.WriteRune(r)
			esc = true
		case r == '"':
			quoted = !quoted
			cur.WriteRune(r)
		case r == delim && !quoted:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	out = append(out, cur.String())
	return out
}

// unquoteFieldName strips the quoting a colon or a delimiter forces on a field
// name. `"file:line"` names the field `file:line`, and a reader that keeps the
// quotes keys every consumer wrong.
//
// The surrounding space is trimmed because toon-go trims it when building row
// keys. Reporting an untrimmed name here would hand callers a Fields entry that
// matches nothing in Rows.
var fieldNameUnescaper = strings.NewReplacer(
	`\"`, `"`, `\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t")

func unquoteFieldName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return fieldNameUnescaper.Replace(s[1 : len(s)-1])
	}
	return s
}

// delimiterSuffix renders a delimiter the way a TOON header declares it. A
// comma is the default and MUST be omitted: toon-go rejects an explicitly
// written one with `invalid delimiter symbol ','`, even though llm-tools has
// shipped payloads containing it since this package was written.
func delimiterSuffix(d rune) string {
	if d == ',' {
		return ""
	}
	return string(d)
}
