package commands

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	encodeEncoding string
	encodeJSON     bool
	encodeMinimal  bool
)

// EncodeResult represents the result of encoding/decoding
type EncodeResult struct {
	Input    string `json:"input,omitempty"`
	I        string `json:"i,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Enc      string `json:"enc,omitempty"`
	Output   string `json:"output,omitempty"`
	O        string `json:"o,omitempty"`
}

// newEncodeCmd creates the encode command
func newEncodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encode [text...]",
		Short: "Encode text using various encodings",
		Long: `Encode text using base64, base32, hex, or URL encoding.

Supported encodings:
  base64 - Base64 encoding (default)
  base32 - Base32 encoding
  hex    - Hexadecimal encoding
  url    - URL encoding`,
		Args: cobra.MinimumNArgs(1),
		RunE: runEncode,
	}
	cmd.Flags().StringVarP(&encodeEncoding, "encoding", "e", "base64", "Encoding type: base64, base32, hex, url")
	cmd.Flags().BoolVar(&encodeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&encodeMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runEncode(cmd *cobra.Command, args []string) error {
	text := strings.Join(args, " ")
	encoding := strings.ToLower(encodeEncoding)

	var result string
	var err error

	switch encoding {
	case "base64":
		result = base64.StdEncoding.EncodeToString([]byte(text))
	case "base32":
		result = base32.StdEncoding.EncodeToString([]byte(text))
	case "hex":
		result = hex.EncodeToString([]byte(text))
	case "url":
		result = url.PathEscape(text) // Use PathEscape for %20 instead of +
	default:
		return fmt.Errorf("unsupported encoding: %s (supported: base64, base32, hex, url)", encoding)
	}

	if err != nil {
		return err
	}

	var encResult EncodeResult
	if encodeMinimal {
		encResult = EncodeResult{
			I:   text,
			Enc: encoding,
			O:   result,
		}
	} else {
		encResult = EncodeResult{
			Input:    text,
			Encoding: encoding,
			Output:   result,
		}
	}

	formatter := output.New(encodeJSON, encodeMinimal, cmd.OutOrStdout())
	return formatter.Print(encResult, func(w io.Writer, data interface{}) {
		r := data.(EncodeResult)
		input := r.Input
		if r.I != "" {
			input = r.I
		}
		enc := r.Encoding
		if r.Enc != "" {
			enc = r.Enc
		}
		out := r.Output
		if r.O != "" {
			out = r.O
		}
		fmt.Fprintf(w, "INPUT: %s\n", input)
		fmt.Fprintf(w, "ENCODING: %s\n", enc)
		fmt.Fprintf(w, "OUTPUT: %s\n", out)
	})
}

var (
	decodeEncoding string
	decodeJSON     bool
	decodeMinimal  bool
)

// newDecodeCmd creates the decode command
func newDecodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decode [text...]",
		Short: "Decode text using various encodings",
		Long: `Decode text using base64, base32, hex, or URL encoding.

Supported encodings:
  base64 - Base64 decoding (default)
  base32 - Base32 decoding
  hex    - Hexadecimal decoding
  url    - URL decoding`,
		Args: cobra.MinimumNArgs(1),
		RunE: runDecode,
	}
	cmd.Flags().StringVarP(&decodeEncoding, "encoding", "e", "base64", "Encoding type: base64, base32, hex, url")
	cmd.Flags().BoolVar(&decodeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&decodeMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runDecode(cmd *cobra.Command, args []string) error {
	text := strings.Join(args, " ")
	encoding := strings.ToLower(decodeEncoding)

	var result string
	var err error
	var decoded []byte

	switch encoding {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(text)
		if err != nil {
			return fmt.Errorf("invalid base64: %v", err)
		}
		result = string(decoded)
	case "base32":
		decoded, err = base32.StdEncoding.DecodeString(text)
		if err != nil {
			return fmt.Errorf("invalid base32: %v", err)
		}
		result = string(decoded)
	case "hex":
		decoded, err = hex.DecodeString(text)
		if err != nil {
			return fmt.Errorf("invalid hex: %v", err)
		}
		result = string(decoded)
	case "url":
		result, err = url.QueryUnescape(text)
		if err != nil {
			return fmt.Errorf("invalid URL encoding: %v", err)
		}
	default:
		return fmt.Errorf("unsupported encoding: %s (supported: base64, base32, hex, url)", encoding)
	}

	var decResult EncodeResult
	if decodeMinimal {
		decResult = EncodeResult{
			I:   text,
			Enc: encoding,
			O:   result,
		}
	} else {
		decResult = EncodeResult{
			Input:    text,
			Encoding: encoding,
			Output:   result,
		}
	}

	formatter := output.New(decodeJSON, decodeMinimal, cmd.OutOrStdout())
	return formatter.Print(decResult, func(w io.Writer, data interface{}) {
		r := data.(EncodeResult)
		input := r.Input
		if r.I != "" {
			input = r.I
		}
		enc := r.Encoding
		if r.Enc != "" {
			enc = r.Enc
		}
		out := r.Output
		if r.O != "" {
			out = r.O
		}
		fmt.Fprintf(w, "INPUT: %s\n", input)
		fmt.Fprintf(w, "ENCODING: %s\n", enc)
		fmt.Fprintf(w, "OUTPUT: %s\n", out)
	})
}

func init() {
	RootCmd.AddCommand(newEncodeCmd())
	RootCmd.AddCommand(newDecodeCmd())
}
