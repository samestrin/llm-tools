package output

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	goaxi "github.com/samestrin/go-axi"
)

// defaultAXI is the AXI setting every Formatter adopts at construction.
//
// It exists because each command builds its own formatter with
// output.New(json, min, w) from its own local flag variables. Threading a
// fourth argument through would mean editing some sixty command files and
// creating sixty chances to miss one — and a command that silently ignored
// --axi would be indistinguishable from one that had no AXI output to give.
// The root command publishes the flag here once, at the single point where
// flags are known to be parsed.
var defaultAXI bool

// SetDefaultAXI publishes the parsed --axi flag to every formatter built after
// it. Called from each CLI's root PersistentPreRun.
func SetDefaultAXI(enabled bool) { defaultAXI = enabled }

// projectJSON renders v through its JSON form.
//
// This is the difference between --axi being a re-encoding of --json and being
// a second, accidental contract. toon-go does NOT fall back to the `json`
// struct tag — probed against the pinned version: encoding a result struct
// directly emits the Go identifiers (`Count`, `Rows{File,Line,Match}`), while
// every result type in this repo carries only `json` tags. Publishing Go field
// names as the column contract for sixty commands would be a decision made by
// accident, and it would break the moment a field was renamed.
//
// Going through JSON also normalises the value to the types toon-go handles
// (maps, slices, strings, float64, bool, nil), so a type that marshals
// specially — time.Time, a json.Marshaler — reaches the encoder in the same
// shape --json would have shown.
func projectJSON(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// UseNumber, because a plain Unmarshal into `any` turns every number into
	// float64 and silently rounds anything past 2^53 — file sizes and byte
	// totals reach that range. Measured before this: --json printed
	// 9007199254740993 while --axi printed 9007199254740992, so the two flags
	// disagreed about the data.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var projected any
	if err := dec.Decode(&projected); err != nil {
		return nil, err
	}
	return exactifyNumbers(projected), nil
}

// maxExactInt is the largest integer float64 represents exactly.
const maxExactInt = 1 << 53

// exactifyNumbers converts the integers float64 cannot hold exactly into their
// verbatim digits, and leaves every other number numeric.
//
// json.Number alone does not solve this: probed, go-axi re-parses it back into
// a number and the digits die again. Carrying only the out-of-range values
// across as text is also what toon-go does natively when it encodes a large
// int64 directly, so this matches the encoder's own convention rather than
// inventing one.
//
// In-range numbers stay numeric deliberately. Rendering every integer as text
// would keep the digits but break --axi's agreement with --json for every
// ordinary payload, which is the far more common case.
func exactifyNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		s := t.String()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			if i > maxExactInt || i < -maxExactInt {
				return s
			}
			return float64(i)
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			if u > maxExactInt {
				return s
			}
			return float64(u)
		}
		f, err := t.Float64()
		if err != nil {
			// Not representable as a float either; the digits are all there is.
			return s
		}
		return f
	case map[string]any:
		for k, vv := range t {
			t[k] = exactifyNumbers(vv)
		}
		return t
	case []any:
		for i, vv := range t {
			t[i] = exactifyNumbers(vv)
		}
		return t
	}
	return v
}

// EncodeAXI writes v to w as TOON, using the field names --json would use.
//
// go-axi sanitizes before encoding, which is load-bearing rather than
// cosmetic: toon-go ERRORS on a raw control byte instead of emitting it, so
// without sanitizing, a command would fail outright on data it should simply
// print — a filename or a matched line carrying an escape sequence.
//
// Encode, not EncodeOrJSON: the output format here is chosen by a flag, and
// runtime format detection would make a command's encoding depend on its
// payload, which no consumer can predict.
func EncodeAXI(w io.Writer, v any) error {
	projected, err := projectJSON(v)
	if err != nil {
		return err
	}
	return goaxi.Encode(w, projected)
}
