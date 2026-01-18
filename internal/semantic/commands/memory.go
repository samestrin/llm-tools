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
	cmd.AddCommand(memoryStatsCmd())

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

// ===== MEMORY STATS COMMAND =====

// displayRow represents a single row in the stats table output
type displayRow struct {
	ID           string
	Question     string
	Created      string
	Retrievals   int
	LastAccessed string
	AvgScore     float32
}

func memoryStatsCmd() *cobra.Command {
	var (
		id            string
		minRetrievals int
		status        string
		tagsFilter    string
		limit         int
		showHistory   bool
		pruneMode     bool
		olderThan     int
		skipConfirm   bool
		jsonOutput    bool
		minOutput     bool
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display retrieval statistics for memories",
		Long: `Display retrieval statistics and history for stored memories.
Shows retrieval counts, last accessed times, and retrieval patterns.

Use --min-retrievals to find frequently accessed memories (promotion candidates).
Use --status to filter by memory status (pending, promoted).
Use --tags to filter by tag (e.g., --tags auth or --tags "sprint:5.0").
Use --history to see detailed retrieval history for a specific memory.
Use --prune to clean up old retrieval log entries.

Examples:
  llm-semantic memory stats
  llm-semantic memory stats --min-retrievals 5
  llm-semantic memory stats --status pending --limit 20
  llm-semantic memory stats --tags auth
  llm-semantic memory stats --id mem-abc123 --history
  llm-semantic memory stats --prune --older-than 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryStats(cmd.Context(), memoryStatsOpts{
				id:            id,
				minRetrievals: minRetrievals,
				status:        status,
				tagsFilter:    tagsFilter,
				limit:         limit,
				showHistory:   showHistory,
				pruneMode:     pruneMode,
				olderThan:     olderThan,
				skipConfirm:   skipConfirm,
				jsonOutput:    jsonOutput,
				minOutput:     minOutput,
			})
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Filter to a specific memory ID")
	cmd.Flags().IntVar(&minRetrievals, "min-retrievals", 0, "Show memories with at least N retrievals")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, promoted)")
	cmd.Flags().StringVar(&tagsFilter, "tags", "", "Filter by tag (e.g., 'auth' or 'sprint:5.0')")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = unlimited)")
	cmd.Flags().BoolVar(&showHistory, "history", false, "Show retrieval history for a memory")
	cmd.Flags().BoolVar(&pruneMode, "prune", false, "Run prune operation")
	cmd.Flags().IntVar(&olderThan, "older-than", 0, "Prune logs older than N days")
	cmd.Flags().BoolVar(&skipConfirm, "yes", false, "Skip confirmation for prune")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	// Require flags together
	cmd.MarkFlagsRequiredTogether("history", "id")
	cmd.MarkFlagsRequiredTogether("prune", "older-than")

	return cmd
}

type memoryStatsOpts struct {
	id            string
	minRetrievals int
	status        string
	tagsFilter    string
	limit         int
	showHistory   bool
	pruneMode     bool
	olderThan     int
	skipConfirm   bool
	jsonOutput    bool
	minOutput     bool
}

func runMemoryStats(ctx context.Context, opts memoryStatsOpts) error {
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

	// Check if backend supports stats tracking
	tracker, ok := storage.(semantic.MemoryStatsTracker)
	if !ok {
		return fmt.Errorf("memory stats not supported by this storage backend")
	}

	// Handle different modes
	if opts.showHistory {
		return runMemoryHistory(ctx, storage, tracker, opts)
	}

	if opts.pruneMode {
		return runMemoryPrune(ctx, tracker, opts)
	}

	// Default: display stats table
	return runMemoryStatsTable(ctx, tracker, storage, opts)
}

func runMemoryStatsTable(ctx context.Context, tracker semantic.MemoryStatsTracker, storage semantic.Storage, opts memoryStatsOpts) error {
	// Validate min-retrievals is non-negative
	if opts.minRetrievals < 0 {
		return fmt.Errorf("invalid value for --min-retrievals: must be a non-negative integer")
	}

	stats, err := tracker.GetAllMemoryStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve memory stats: %w", err)
	}

	// Count totals before filtering
	totalMemories := len(stats)
	pendingCount := 0
	promotedCount := 0
	for _, stat := range stats {
		if stat.Status == "promoted" {
			promotedCount++
		} else {
			pendingCount++
		}
	}

	// Build filtered results
	var filtered []semantic.RetrievalStats
	for _, stat := range stats {
		// Filter by minimum retrievals
		if stat.RetrievalCount < opts.minRetrievals {
			continue
		}

		// Filter by ID if specified
		if opts.id != "" && stat.MemoryID != opts.id {
			continue
		}

		// Filter by status if specified
		if opts.status != "" && stat.Status != opts.status {
			continue
		}

		// Filter by tags if specified
		if opts.tagsFilter != "" {
			found := false
			for _, tag := range stat.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(opts.tagsFilter)) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, stat)

		// Apply limit if specified
		if opts.limit > 0 && len(filtered) >= opts.limit {
			break
		}
	}

	// Handle JSON output
	if opts.jsonOutput || opts.minOutput {
		enc := json.NewEncoder(os.Stdout)
		if !opts.minOutput {
			enc.SetIndent("", "  ")
		}

		if opts.minOutput {
			// Minimal JSON output with abbreviated keys
			minResults := make([]map[string]interface{}, len(filtered))
			for i, stat := range filtered {
				minResults[i] = map[string]interface{}{
					"id": stat.MemoryID,
					"q":  truncateRuneAware(stat.Question, 50),
					"r":  stat.RetrievalCount,
					"s":  stat.Status,
				}
				if stat.AvgScore > 0 {
					minResults[i]["a"] = stat.AvgScore
				}
			}
			output := map[string]interface{}{
				"t": totalMemories,
				"p": pendingCount,
				"m": promotedCount,
				"r": minResults,
			}
			return enc.Encode(output)
		}

		// Full JSON output with summary
		results := make([]map[string]interface{}, len(filtered))
		for i, stat := range filtered {
			results[i] = map[string]interface{}{
				"id":              stat.MemoryID,
				"question":        stat.Question,
				"retrieval_count": stat.RetrievalCount,
				"last_retrieved":  stat.LastRetrieved,
				"avg_score":       stat.AvgScore,
				"status":          stat.Status,
				"created":         stat.CreatedAt,
				"tags":            stat.Tags,
			}
		}
		output := map[string]interface{}{
			"total_memories": totalMemories,
			"pending":        pendingCount,
			"promoted":       promotedCount,
			"results":        results,
		}
		return enc.Encode(output)
	}

	// Build display rows for table output
	var rows []displayRow
	for _, stat := range filtered {
		question := "<entry not found>"
		created := "Unknown"
		if stat.Question != "" {
			question = stat.Question
		}
		if stat.CreatedAt != "" {
			created = formatDisplayDate(stat.CreatedAt)
		}

		lastAccessed := "Never"
		if stat.LastRetrieved != "" {
			lastAccessed = formatDisplayDate(stat.LastRetrieved)
		}

		rows = append(rows, displayRow{
			ID:           stat.MemoryID,
			Question:     question,
			Created:      created,
			Retrievals:   stat.RetrievalCount,
			LastAccessed: lastAccessed,
			AvgScore:     stat.AvgScore,
		})
	}

	if len(rows) == 0 {
		if opts.id != "" || opts.minRetrievals > 0 || opts.status != "" || opts.tagsFilter != "" {
			fmt.Println("No memories found matching filters.")
		} else {
			fmt.Println("No memories found.")
		}
		return nil
	}

	// Print summary
	fmt.Printf("Total: %d | Pending: %d | Promoted: %d\n\n", totalMemories, pendingCount, promotedCount)

	// Display table
	displayStatsTable(rows)
	return nil
}

func displayStatsTable(rows []displayRow) {
	// Truncate questions to 60 chars (rune-aware for UTF-8)
	const maxQuestionLen = 60

	// Print header
	fmt.Printf("%-14s  %-60s  %-12s  %-10s  %-16s  %-8s\n",
		"ID", "Question", "Created", "Retrievals", "Last Accessed", "Avg Score")
	fmt.Println(strings.Repeat("-", 130))

	// Print rows
	for _, row := range rows {
		question := truncateRuneAware(row.Question, maxQuestionLen-3)

		avgScoreStr := ""
		if row.AvgScore > 0 {
			avgScoreStr = fmt.Sprintf("%.4f", row.AvgScore)
		}

		fmt.Printf("%-14s  %-60s  %-12s  %-10d  %-16s  %-8s\n",
			row.ID, question, row.Created, row.Retrievals, row.LastAccessed, avgScoreStr)
	}
}

// truncateRuneAware truncates string to max runes, preserving UTF-8 boundaries
func truncateRuneAware(s string, maxRunes int) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

func runMemoryHistory(ctx context.Context, storage semantic.Storage, tracker semantic.MemoryStatsTracker, opts memoryStatsOpts) error {
	// Verify memory exists
	mem, err := storage.GetMemory(ctx, opts.id)
	if err != nil {
		return fmt.Errorf("memory not found: %s (use 'llm-semantic memory list' to find IDs): %w", opts.id, err)
	}

	history, err := tracker.GetMemoryRetrievalHistory(ctx, opts.id, 50)
	if err != nil {
		return fmt.Errorf("failed to retrieve history: %w", err)
	}

	// Handle JSON output
	if opts.jsonOutput || opts.minOutput {
		enc := json.NewEncoder(os.Stdout)
		if !opts.minOutput {
			enc.SetIndent("", "  ")
		}

		if opts.minOutput {
			// Minimal JSON output with abbreviated keys
			minEntries := make([]map[string]interface{}, len(history))
			for i, entry := range history {
				minEntries[i] = map[string]interface{}{
					"t": entry.Timestamp,
					"q": truncateRuneAware(entry.Query, 47),
					"s": entry.Score,
				}
			}
			result := map[string]interface{}{
				"i":  mem.ID,
				"q":  truncateRuneAware(mem.Question, 100),
				"rc": len(history),
				"h":  minEntries,
			}
			return enc.Encode(result)
		}

		// Full JSON output
		result := map[string]interface{}{
			"memory_id":     mem.ID,
			"question":      mem.Question,
			"history_count": len(history),
			"history":       history,
		}
		return enc.Encode(result)
	}

	fmt.Printf("Memory: %s\n", mem.Question)
	fmt.Printf("Retrieval History (%d entries):\n\n", len(history))

	if len(history) == 0 {
		fmt.Println("No retrieval history found.")
		return nil
	}

	// Print header
	fmt.Printf("%-26s  %-50s  %-10s\n", "Timestamp", "Query", "Score")
	fmt.Println(strings.Repeat("-", 90))

	// Print history entries
	for _, entry := range history {
		// Truncate query (rune-aware) to 50 chars with ellipsis
		query := truncateRuneAware(entry.Query, 47)

		timestamp := entry.Timestamp
		if timestamp == "" {
			timestamp = "Unknown"
		}

		fmt.Printf("%-26s  %-50s  %-10.4f\n", timestamp, query, entry.Score)
	}

	return nil
}

func runMemoryPrune(ctx context.Context, tracker semantic.MemoryStatsTracker, opts memoryStatsOpts) error {
	// Validate older-than is positive (also cap at 100 years = 36500 days)
	if opts.olderThan <= 0 {
		return fmt.Errorf("invalid value for --older-than: must be a positive integer")
	}
	if opts.olderThan > 36500 {
		return fmt.Errorf("invalid value for --older-than: must be <= 36500 (100 years)")
	}

	// Confirm deletion
	if !opts.skipConfirm {
		fmt.Printf("This will delete retrieval log entries older than %d days.\n", opts.olderThan)
		fmt.Print("Are you sure? [y/N]: ")

		var response string
		_, err := fmt.Scanln(&response)
		// If stdin is closed/redirected, abort (require explicit --yes for automation)
		if err != nil {
			fmt.Println("Aborted (non-interactive input). Use --yes to confirm automatically.")
			return nil
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	count, err := tracker.PruneMemoryRetrievalLog(ctx, opts.olderThan)
	if err != nil {
		return fmt.Errorf("failed to prune retrieval log: %w", err)
	}

	// Handle JSON output
	if opts.jsonOutput || opts.minOutput {
		enc := json.NewEncoder(os.Stdout)
		if !opts.minOutput {
			enc.SetIndent("", "  ")
		}

		if opts.minOutput {
			result := map[string]interface{}{
				"c": count,
				"o": opts.olderThan,
			}
			return enc.Encode(result)
		}

		result := map[string]interface{}{
			"deleted_count":   count,
			"older_than_days": opts.olderThan,
		}
		return enc.Encode(result)
	}

	if count > 0 {
		fmt.Printf("Deleted %d log entries older than %d days.\n", count, opts.olderThan)
	} else {
		fmt.Println("No log entries to delete.")
	}

	return nil
}

// Helper function to format display date from ISO format to readable format
func formatDisplayDate(isoDate string) string {
	if isoDate == "" {
		return "Unknown"
	}
	// Parse ISO 8601 format and return first 16 chars (YYYY-MM-DD HH:MM)
	if len(isoDate) >= 16 {
		// Replace T with space for display
		return isoDate[:10] + " " + isoDate[11:16]
	}
	return isoDate
}
