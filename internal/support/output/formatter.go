package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Format represents the output format type
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Formatter handles output formatting for the CLI
type Formatter struct {
	format Format
}

// NewFormatter creates a new output formatter
func NewFormatter(format string) *Formatter {
	f := Format(format)
	if f != FormatText && f != FormatJSON {
		f = FormatText
	}
	return &Formatter{format: f}
}

// Print outputs data in the configured format
func (f *Formatter) Print(data interface{}) {
	switch f.format {
	case FormatJSON:
		f.printJSON(data)
	default:
		f.printText(data)
	}
}

// PrintKeyValue prints a key-value pair in the standard format
func (f *Formatter) PrintKeyValue(key string, value interface{}) {
	if f.format == FormatJSON {
		data := map[string]interface{}{key: value}
		f.printJSON(data)
	} else {
		fmt.Printf("%s: %v\n", strings.ToUpper(key), value)
	}
}

// PrintError prints an error message to stderr
func (f *Formatter) PrintError(msg string) {
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
}

// PrintSuccess prints a success indicator
func (f *Formatter) PrintSuccess(msg string) {
	fmt.Println("✓", msg)
}

// PrintFailure prints a failure indicator
func (f *Formatter) PrintFailure(msg string) {
	fmt.Println("✗", msg)
}

func (f *Formatter) printJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

func (f *Formatter) printText(data interface{}) {
	fmt.Printf("%v\n", data)
}
