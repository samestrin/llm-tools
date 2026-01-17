package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func memoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage learned decisions and clarifications",
		Long: `Memory commands for storing, searching, and managing
learned decisions and clarifications in the semantic database.

Commands:
  store   - Store a question/answer memory
  search  - Semantic search of stored memories
  promote - Append a memory entry to CLAUDE.md
  import  - Migrate from clarification-tracking.yaml`,
	}

	cmd.AddCommand(memoryStoreCmd())
	cmd.AddCommand(memorySearchCmd())
	cmd.AddCommand(memoryPromoteCmd())
	cmd.AddCommand(memoryImportCmd())
	cmd.AddCommand(memoryListCmd())
	cmd.AddCommand(memoryDeleteCmd())

	return cmd
}

// ===== MEMORY STORE COMMAND =====

func memoryStoreCmd() *cobra.Command {
	var (
		question   string
		answer     string
		tags       string
		source     string
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "store",
		Short: "Store a question/answer memory",
		Long: `Store a learned decision or clarification in the semantic database.
The question and answer text is embedded for later semantic search.

Example:
  llm-semantic memory store -q "How should auth tokens be handled?" -a "Use JWT with 24h expiry"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if question == "" || answer == "" {
				return fmt.Errorf("--question and --answer are required")
			}
			return runMemoryStore(cmd.Context(), memoryStoreOpts{
				question:   question,
				answer:     answer,
				tags:       tags,
				source:     source,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().StringVarP(&question, "question", "q", "", "Question or decision (required)")
	cmd.Flags().StringVarP(&answer, "answer", "a", "", "Answer or decision made (required)")
	cmd.Flags().StringVarP(&tags, "tags", "t", "", "Comma-separated context tags")
	cmd.Flags().StringVarP(&source, "source", "s", "manual", "Origin source")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("question")
	cmd.MarkFlagRequired("answer")

	return cmd
}

type memoryStoreOpts struct {
	question   string
	answer     string
	tags       string
	source     string
	jsonOutput bool
	minOutput  bool
}

func runMemoryStore(ctx context.Context, opts memoryStoreOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			// Create default index path
			indexPath = ".index/semantic.db"
		}
	}

	// Create embedder
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// Get embedding dimension
	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Create memory entry
	entry := semantic.NewMemoryEntry(opts.question, opts.answer)
	if opts.tags != "" {
		entry.Tags = strings.Split(opts.tags, ",")
		for i := range entry.Tags {
			entry.Tags[i] = strings.TrimSpace(entry.Tags[i])
		}
	}
	entry.Source = opts.source

	// Generate embedding
	embedding, err := embedder.Embed(ctx, entry.EmbeddingText())
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Store the entry
	if err := storage.StoreMemory(ctx, *entry, embedding); err != nil {
		return fmt.Errorf("failed to store memory: %w", err)
	}

	// Output result
	if opts.jsonOutput || opts.minOutput {
		result := map[string]interface{}{
			"status": "stored",
			"id":     entry.ID,
		}
		if !opts.minOutput {
			result["question"] = entry.Question
			result["answer"] = entry.Answer
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Memory stored: %s\n", entry.ID)
	return nil
}

// ===== MEMORY SEARCH COMMAND =====

func memorySearchCmd() *cobra.Command {
	var (
		topK       int
		threshold  float64
		tags       string
		status     string
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Semantic search of stored memories",
		Long: `Search stored memories using natural language queries.
Returns ranked results based on semantic similarity.

Example:
  llm-semantic memory search "authentication" --top 5`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runMemorySearch(cmd.Context(), memorySearchOpts{
				query:      query,
				topK:       topK,
				threshold:  float32(threshold),
				tags:       tags,
				status:     status,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().IntVarP(&topK, "top", "n", 10, "Number of results to return")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.0, "Minimum similarity score (0.0-1.0)")
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by tags (comma-separated)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, promoted)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Output minimal JSON format")

	return cmd
}

type memorySearchOpts struct {
	query      string
	topK       int
	threshold  float32
	tags       string
	status     string
	jsonOutput bool
	minOutput  bool
}

func runMemorySearch(ctx context.Context, opts memorySearchOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic memory store' first")
		}
	}

	// Create embedder
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// Get embedding dimension
	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Generate query embedding
	queryEmbedding, err := embedder.Embed(ctx, opts.query)
	if err != nil {
		return fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Parse tags filter
	var tagsList []string
	if opts.tags != "" {
		tagsList = strings.Split(opts.tags, ",")
		for i := range tagsList {
			tagsList[i] = strings.TrimSpace(tagsList[i])
		}
	}

	// Search memories
	results, err := storage.SearchMemory(ctx, queryEmbedding, semantic.MemorySearchOptions{
		TopK:      opts.topK,
		Threshold: opts.threshold,
		Tags:      tagsList,
		Status:    semantic.MemoryStatus(opts.status),
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Track retrieval stats (automatic for memory profile)
	if len(results) > 0 {
		if tracker, ok := storage.(semantic.MemoryStatsTracker); ok {
			// Build retrieval batch
			retrievals := make([]semantic.MemoryRetrieval, len(results))
			for i, r := range results {
				retrievals[i] = semantic.MemoryRetrieval{
					MemoryID: r.Entry.ID,
					Score:    r.Score,
				}
			}
			// Track in background - don't fail search if tracking fails
			if err := tracker.TrackMemoryRetrievalBatch(ctx, retrievals, opts.query); err != nil {
				// Log error but don't fail the search
				fmt.Fprintf(os.Stderr, "Warning: failed to track retrieval stats: %v\n", err)
			}
		}
	}

	// Output results
	if opts.jsonOutput || opts.minOutput {
		return outputMemoryJSON(results, opts.minOutput)
	}
	return outputMemoryText(results)
}

func outputMemoryJSON(results []semantic.MemorySearchResult, minimal bool) error {
	if minimal {
		minResults := make([]map[string]interface{}, len(results))
		for i, r := range results {
			minResults[i] = map[string]interface{}{
				"id": r.Entry.ID,
				"q":  truncate(r.Entry.Question, 50),
				"s":  r.Score,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(minResults)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func outputMemoryText(results []semantic.MemorySearchResult) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range results {
		fmt.Printf("%d. [%s] Score: %.4f\n", i+1, r.Entry.ID, r.Score)
		fmt.Printf("   Q: %s\n", r.Entry.Question)
		fmt.Printf("   A: %s\n", r.Entry.Answer)
		if len(r.Entry.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(r.Entry.Tags, ", "))
		}
		fmt.Println()
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ===== MEMORY PROMOTE COMMAND =====

func memoryPromoteCmd() *cobra.Command {
	var (
		target     string
		section    string
		force      bool
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Append a memory entry to CLAUDE.md",
		Long: `Promote a memory entry by appending it to a CLAUDE.md file.
Updates the entry's status to 'promoted'.

Example:
  llm-semantic memory promote mem-abc123 --target ./CLAUDE.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryPromote(cmd.Context(), memoryPromoteOpts{
				id:         args[0],
				target:     target,
				section:    section,
				force:      force,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Target CLAUDE.md file path (required)")
	cmd.Flags().StringVar(&section, "section", "Learned Clarifications", "Section header to append under")
	cmd.Flags().BoolVar(&force, "force", false, "Re-promote even if already promoted")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("target")

	return cmd
}

type memoryPromoteOpts struct {
	id         string
	target     string
	section    string
	force      bool
	jsonOutput bool
	minOutput  bool
}

func runMemoryPromote(ctx context.Context, opts memoryPromoteOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic memory store' first")
		}
	}

	// Create embedder for storage dimension
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Get the memory entry
	entry, err := storage.GetMemory(ctx, opts.id)
	if err != nil {
		return fmt.Errorf("failed to get memory: %w", err)
	}

	// Check if already promoted
	if entry.Status == semantic.MemoryStatusPromoted && !opts.force {
		return fmt.Errorf("memory %s is already promoted. Use --force to re-promote", opts.id)
	}

	// Format as markdown
	markdown := fmt.Sprintf("\n- **Q:** %s **A:** %s", entry.Question, entry.Answer)
	if len(entry.Tags) > 0 {
		markdown += fmt.Sprintf(" _(Tags: %s)_", strings.Join(entry.Tags, ", "))
	}
	markdown += "\n"

	// Read existing file or create new
	content, err := os.ReadFile(opts.target)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read target file: %w", err)
	}

	// Find or create section
	sectionHeader := fmt.Sprintf("## %s", opts.section)
	fileContent := string(content)
	if !strings.Contains(fileContent, sectionHeader) {
		// Append section at the end
		fileContent += fmt.Sprintf("\n%s\n", sectionHeader)
	}

	// Insert entry under section
	sectionIdx := strings.Index(fileContent, sectionHeader)
	insertIdx := sectionIdx + len(sectionHeader)
	// Find end of line
	for insertIdx < len(fileContent) && fileContent[insertIdx] != '\n' {
		insertIdx++
	}
	if insertIdx < len(fileContent) {
		insertIdx++ // Move past newline
	}

	// Insert markdown
	fileContent = fileContent[:insertIdx] + markdown + fileContent[insertIdx:]

	// Write back
	if err := os.WriteFile(opts.target, []byte(fileContent), 0644); err != nil {
		return fmt.Errorf("failed to write target file: %w", err)
	}

	// Update entry status
	entry.Status = semantic.MemoryStatusPromoted
	embedding, _ := embedder.Embed(ctx, entry.EmbeddingText())
	if err := storage.StoreMemory(ctx, *entry, embedding); err != nil {
		// Log but don't fail - the promotion succeeded
		fmt.Fprintf(os.Stderr, "Warning: failed to update memory status: %v\n", err)
	}

	// Output result
	if opts.jsonOutput || opts.minOutput {
		result := map[string]interface{}{
			"status":   "promoted",
			"id":       entry.ID,
			"target":   opts.target,
			"promoted": true,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Promoted memory %s to %s\n", opts.id, opts.target)
	return nil
}

// ===== MEMORY IMPORT COMMAND =====

func memoryImportCmd() *cobra.Command {
	var (
		source     string
		dryRun     bool
		force      bool
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Migrate from clarification-tracking.yaml",
		Long: `Import clarifications from an existing YAML file into the semantic memory.
Generates embeddings for each entry and stores them for semantic search.

Example:
  llm-semantic memory import --source ./clarification-tracking.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryImport(cmd.Context(), memoryImportOpts{
				source:     source,
				dryRun:     dryRun,
				force:      force,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Path to clarification YAML file (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without writing")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing entries with same ID")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("source")

	return cmd
}

type memoryImportOpts struct {
	source     string
	dryRun     bool
	force      bool
	jsonOutput bool
	minOutput  bool
}

// ClarificationEntry represents an entry in clarification-tracking.yaml
type ClarificationEntry struct {
	ID                string   `yaml:"id"`
	CanonicalQuestion string   `yaml:"canonical_question"`
	CurrentAnswer     string   `yaml:"current_answer"`
	ContextTags       []string `yaml:"context_tags"`
	SprintID          string   `yaml:"sprint_id"`
	Occurrences       int      `yaml:"occurrences"`
	Status            string   `yaml:"status"`
	CreatedAt         string   `yaml:"created_at"`
	UpdatedAt         string   `yaml:"updated_at"`
}

// ClarificationFile represents the structure of clarification-tracking.yaml
type ClarificationFile struct {
	Entries []ClarificationEntry `yaml:"entries"`
}

func runMemoryImport(ctx context.Context, opts memoryImportOpts) error {
	// Read and parse YAML
	data, err := os.ReadFile(opts.source)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	var clarFile ClarificationFile
	if err := yaml.Unmarshal(data, &clarFile); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(clarFile.Entries) == 0 {
		fmt.Println("No entries found in source file.")
		return nil
	}

	if opts.dryRun {
		fmt.Printf("Dry run: would import %d entries\n", len(clarFile.Entries))
		for i, e := range clarFile.Entries {
			fmt.Printf("  %d. [%s] %s\n", i+1, e.ID, truncate(e.CanonicalQuestion, 60))
		}
		return nil
	}

	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			// Create default index path
			indexPath = ".index/semantic.db"
		}
	}

	// Create embedder
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// Get embedding dimension
	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Import entries
	var imported, skipped, errors int
	for i, clar := range clarFile.Entries {
		// Convert to MemoryEntry
		entry := &semantic.MemoryEntry{
			ID:          clar.ID,
			Question:    clar.CanonicalQuestion,
			Answer:      clar.CurrentAnswer,
			Tags:        clar.ContextTags,
			Source:      clar.SprintID,
			Occurrences: clar.Occurrences,
			CreatedAt:   clar.CreatedAt,
			UpdatedAt:   clar.UpdatedAt,
		}
		if entry.ID == "" {
			entry.ID = entry.GenerateID()
		}
		if clar.Status == "promoted" {
			entry.Status = semantic.MemoryStatusPromoted
		} else {
			entry.Status = semantic.MemoryStatusPending
		}

		// Check if exists
		if !opts.force {
			_, err := storage.GetMemory(ctx, entry.ID)
			if err == nil {
				skipped++
				continue
			}
		}

		// Generate embedding
		embedding, err := embedder.Embed(ctx, entry.EmbeddingText())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to embed entry %d: %v\n", i+1, err)
			errors++
			continue
		}

		// Store
		if err := storage.StoreMemory(ctx, *entry, embedding); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to store entry %d: %v\n", i+1, err)
			errors++
			continue
		}

		imported++
	}

	// Output result
	if opts.jsonOutput || opts.minOutput {
		result := map[string]interface{}{
			"imported": imported,
			"skipped":  skipped,
			"errors":   errors,
			"total":    len(clarFile.Entries),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Import complete: %d imported, %d skipped, %d errors (total: %d)\n",
		imported, skipped, errors, len(clarFile.Entries))
	return nil
}

// ===== MEMORY LIST COMMAND =====

func memoryListCmd() *cobra.Command {
	var (
		limit      int
		status     string
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored memories",
		Long: `List all stored memories with optional filtering.

Example:
  llm-semantic memory list --limit 20 --status pending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryList(cmd.Context(), memoryListOpts{
				limit:      limit,
				status:     status,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of entries to return")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, promoted)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	return cmd
}

type memoryListOpts struct {
	limit      int
	status     string
	jsonOutput bool
	minOutput  bool
}

func runMemoryList(ctx context.Context, opts memoryListOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic memory store' first")
		}
	}

	// Create embedder for storage dimension
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// List memories
	entries, err := storage.ListMemory(ctx, semantic.MemoryListOptions{
		Limit:  opts.limit,
		Status: semantic.MemoryStatus(opts.status),
	})
	if err != nil {
		return fmt.Errorf("failed to list memories: %w", err)
	}

	// Output results
	if opts.jsonOutput || opts.minOutput {
		if opts.minOutput {
			minResults := make([]map[string]interface{}, len(entries))
			for i, e := range entries {
				minResults[i] = map[string]interface{}{
					"id":     e.ID,
					"q":      truncate(e.Question, 50),
					"status": e.Status,
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(minResults)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(entries) == 0 {
		fmt.Println("No memories found.")
		return nil
	}

	for i, e := range entries {
		fmt.Printf("%d. [%s] (%s)\n", i+1, e.ID, e.Status)
		fmt.Printf("   Q: %s\n", e.Question)
		fmt.Printf("   A: %s\n", e.Answer)
		if len(e.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(e.Tags, ", "))
		}
		fmt.Println()
	}

	return nil
}

// ===== MEMORY DELETE COMMAND =====

func memoryDeleteCmd() *cobra.Command {
	var (
		force      bool
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a memory entry",
		Long: `Delete a memory entry by ID.

Example:
  llm-semantic memory delete mem-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryDelete(cmd.Context(), memoryDeleteOpts{
				id:         args[0],
				force:      force,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	return cmd
}

type memoryDeleteOpts struct {
	id         string
	force      bool
	jsonOutput bool
	minOutput  bool
}

func runMemoryDelete(ctx context.Context, opts memoryDeleteOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic memory store' first")
		}
	}

	// Create embedder for storage dimension
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Delete memory
	if err := storage.DeleteMemory(ctx, opts.id); err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	// Output result
	if opts.jsonOutput || opts.minOutput {
		result := map[string]interface{}{
			"status":  "deleted",
			"id":      opts.id,
			"deleted": true,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Deleted memory: %s\n", opts.id)
	return nil
}
