package output

import (
	"encoding/json"
	"io"

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
	var projected any
	if err := json.Unmarshal(b, &projected); err != nil {
		return nil, err
	}
	return projected, nil
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
