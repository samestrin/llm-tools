package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
)

func callersCmd() *cobra.Command {
	var (
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "callers <symbol>",
		Short: "Show what calls a symbol",
		Long: `List the functions and methods that call a symbol.

Answers come from the call graph recorded while indexing, so the index has to be
current. A call is only attributed when exactly one indexed symbol carries the
name, because a confidently wrong answer is worse than none.

Example:
  llm-semantic callers ResolveRefs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCallers(cmd.Context(), args[0], jsonOutput, minOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	return cmd
}

func refsCmd() *cobra.Command {
	var (
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "refs <symbol>",
		Short: "Show what a symbol references",
		Long: `List the calls, imports and type references made by a symbol.

References to something outside the index, such as a standard library call, are
listed and marked external rather than dropped.

Example:
  llm-semantic refs ResolveRefs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefs(cmd.Context(), args[0], jsonOutput, minOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	return cmd
}

func runCallers(ctx context.Context, symbol string, jsonOutput, minOutput bool) error {
	rs, cleanup, err := openRefStorage(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	edges, err := rs.GetCallersByName(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to look up callers of %q: %w", symbol, err)
	}

	return formatCallers(os.Stdout, symbol, edges, jsonOutput, minOutput)
}

func runRefs(ctx context.Context, symbol string, jsonOutput, minOutput bool) error {
	rs, cleanup, err := openRefStorage(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	edges, err := rs.GetRefsByName(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to look up references from %q: %w", symbol, err)
	}

	return formatRefs(os.Stdout, symbol, edges, jsonOutput, minOutput)
}

// openRefStorage opens the configured backend and returns its call graph view,
// along with a cleanup function the caller must invoke.
func openRefStorage(ctx context.Context) (semantic.RefStorage, func(), error) {
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return nil, nil, fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
		}
	}

	embedder, err := createEmbedder()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddingDim, err := probeEmbeddingDim(ctx, embedder)
	if err != nil {
		return nil, nil, err
	}

	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open index: %w", err)
	}

	rs, ok := storage.(semantic.RefStorage)
	if !ok {
		storage.Close()
		return nil, nil, fmt.Errorf("storage backend %q does not record a call graph", storageType)
	}

	return rs, func() { storage.Close() }, nil
}

func formatCallers(w io.Writer, symbol string, edges []semantic.RefEdge, jsonOutput, minOutput bool) error {
	if jsonOutput || minOutput {
		return writeCallersJSON(w, symbol, edges, minOutput)
	}

	if len(edges) == 0 {
		fmt.Fprintf(w, "No callers of %s (the symbol may not be indexed)\n", symbol)
		return nil
	}

	fmt.Fprintf(w, "%d caller(s) of %s\n\n", len(edges), symbol)
	for _, e := range edges {
		fmt.Fprintf(w, "  %s:%d  %s\n", e.FilePath, e.StartLine, e.Name)
	}
	return nil
}

func writeCallersJSON(w io.Writer, symbol string, edges []semantic.RefEdge, minimal bool) error {
	callers := make([]map[string]interface{}, 0, len(edges))
	for _, e := range edges {
		entry := map[string]interface{}{
			"file": e.FilePath,
			"line": e.StartLine,
			"name": e.Name,
		}
		if !minimal {
			entry["type"] = string(e.RefType)
			entry["ref"] = e.RefName
		}
		callers = append(callers, entry)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]interface{}{
		"symbol":  symbol,
		"count":   len(callers),
		"callers": callers,
	})
}

func formatRefs(w io.Writer, symbol string, edges []semantic.RefEdge, jsonOutput, minOutput bool) error {
	if jsonOutput || minOutput {
		return writeRefsJSON(w, symbol, edges, minOutput)
	}

	if len(edges) == 0 {
		fmt.Fprintf(w, "No references from %s (the symbol may not be indexed)\n", symbol)
		return nil
	}

	fmt.Fprintf(w, "%d reference(s) from %s\n\n", len(edges), symbol)
	for _, e := range edges {
		if e.Resolved() {
			fmt.Fprintf(w, "  %-10s %-24s %s:%d\n", e.RefType, e.RefName, e.FilePath, e.StartLine)
			continue
		}
		fmt.Fprintf(w, "  %-10s %-24s (external)\n", e.RefType, e.RefName)
	}
	return nil
}

func writeRefsJSON(w io.Writer, symbol string, edges []semantic.RefEdge, minimal bool) error {
	refs := make([]map[string]interface{}, 0, len(edges))
	for _, e := range edges {
		entry := map[string]interface{}{
			"ref":      e.RefName,
			"type":     string(e.RefType),
			"resolved": e.Resolved(),
		}
		if e.Resolved() {
			entry["file"] = e.FilePath
			entry["line"] = e.StartLine
			if !minimal {
				entry["name"] = e.Name
			}
		}
		refs = append(refs, entry)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]interface{}{
		"symbol":     symbol,
		"count":      len(refs),
		"references": refs,
	})
}
