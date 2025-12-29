package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var clusterClarificationsCmd = &cobra.Command{
	Use:   "cluster-clarifications",
	Short: "Group semantically similar questions together",
	Long:  `Takes a list of questions and clusters them by semantic similarity. Useful for identifying duplicate or related clarifications.`,
	RunE:  runClusterClarifications,
}

var (
	clusterFile    string
	clusterJSON    bool
	clusterMinimal bool
)

func init() {
	rootCmd.AddCommand(clusterClarificationsCmd)
	clusterClarificationsCmd.Flags().StringVarP(&clusterFile, "file", "f", "", "Tracking file path (required)")
	clusterClarificationsCmd.Flags().BoolVar(&clusterJSON, "json", false, "Output as JSON")
	clusterClarificationsCmd.Flags().BoolVar(&clusterMinimal, "min", false, "Output in minimal/token-optimized format")
	clusterClarificationsCmd.MarkFlagRequired("file")
}

// Cluster represents a group of similar questions.
type Cluster struct {
	Label           string   `json:"label"`
	QuestionIndices []int    `json:"question_indices"`
	Questions       []string `json:"questions,omitempty"`
}

// ClusterResult represents the JSON output of the cluster-clarifications command.
type ClusterResult struct {
	Status       string    `json:"status"`
	Clusters     []Cluster `json:"clusters"`
	ClusterCount int       `json:"cluster_count"`
	Note         string    `json:"note,omitempty"`
}

func runClusterClarifications(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, clusterFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Extract questions from entries
	questions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.CanonicalQuestion != "" {
			questions = append(questions, entry.CanonicalQuestion)
		}
	}

	// Need at least 2 questions to cluster
	if len(questions) < 2 {
		clusters := make([]Cluster, 0, len(questions))
		for i, q := range questions {
			clusters = append(clusters, Cluster{
				Label:           "Unique",
				QuestionIndices: []int{i + 1},
				Questions:       []string{q},
			})
		}
		result := ClusterResult{
			Status:       "insufficient_data",
			Clusters:     clusters,
			ClusterCount: len(clusters),
			Note:         "Not enough questions to cluster",
		}
		formatter := output.New(clusterJSON, clusterMinimal, cmd.OutOrStdout())
		return formatter.Print(result, printClusterText)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildClusterPrompt(questions)

	// Call LLM
	response, err := client.Complete(prompt, 30*time.Second)
	if err != nil {
		return fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse response
	cleanedResponse, err := llmapi.ExtractJSON(response)
	if err != nil {
		return fmt.Errorf("failed to parse LLM response: %w", err)
	}

	var llmResult struct {
		Clusters     []Cluster `json:"clusters"`
		ClusterCount int       `json:"cluster_count"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	// Enrich clusters with actual questions
	for i := range llmResult.Clusters {
		llmResult.Clusters[i].Questions = make([]string, 0, len(llmResult.Clusters[i].QuestionIndices))
		for _, idx := range llmResult.Clusters[i].QuestionIndices {
			if idx > 0 && idx <= len(questions) {
				llmResult.Clusters[i].Questions = append(llmResult.Clusters[i].Questions, questions[idx-1])
			}
		}
	}

	result := ClusterResult{
		Status:       "clustered",
		Clusters:     llmResult.Clusters,
		ClusterCount: llmResult.ClusterCount,
	}

	formatter := output.New(clusterJSON, clusterMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printClusterText)
}

func printClusterText(w io.Writer, data interface{}) {
	r := data.(ClusterResult)
	fmt.Fprintf(w, "STATUS: %s\n", r.Status)
	fmt.Fprintf(w, "CLUSTER_COUNT: %d\n", r.ClusterCount)
	if r.Note != "" {
		fmt.Fprintf(w, "NOTE: %s\n", r.Note)
	}
	for i, c := range r.Clusters {
		fmt.Fprintf(w, "\nCLUSTER %d: %s\n", i+1, c.Label)
		for _, q := range c.Questions {
			fmt.Fprintf(w, "  - %s\n", q)
		}
	}
}

func buildClusterPrompt(questions []string) string {
	// Format questions with numbers
	numberedQuestions := ""
	for i, q := range questions {
		numberedQuestions += fmt.Sprintf("%d. \"%s\"\n", i+1, q)
	}

	return fmt.Sprintf(`Cluster the following questions by semantic similarity.
Questions that ask about the same topic or decision should be in the same cluster.

QUESTIONS:
%s
INSTRUCTIONS:
- Group questions that are semantically similar (asking about the same thing)
- Each cluster should have a descriptive label
- Questions that are unique should be in their own cluster
- Return question numbers (1-indexed) in each cluster

Return ONLY valid JSON in this exact format (no markdown, no explanation):
{"clusters": [{"label": "<cluster description>", "question_indices": [<1-indexed numbers>]}], "cluster_count": <number>}`, numberedQuestions)
}
