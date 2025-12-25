package commands

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNormalizeClarificationCmd_Success(t *testing.T) {
	// Reset flags
	normalizeQuestion = ""

	cmd := newTestRootCmd()
	normalizeCmd := *normalizeClarificationCmd
	normalizeCmd.ResetFlags()
	normalizeCmd.Flags().StringVarP(&normalizeQuestion, "question", "q", "", "Question to normalize")
	cmd.AddCommand(&normalizeCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"normalize-clarification", "-q", "what framework we should use?"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"normalized_question": "What framework should we use?", "changes": "Capitalized, added question mark"}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("normalize-clarification failed: %v", err)
	}

	// Verify JSON output
	var result NormalizeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "normalized" {
		t.Errorf("expected status 'normalized', got %s", result.Status)
	}
	if result.NormalizedQuestion != "What framework should we use?" {
		t.Errorf("expected normalized question 'What framework should we use?', got %s", result.NormalizedQuestion)
	}
}

func TestNormalizeClarificationCmd_RequiredFlag(t *testing.T) {
	cmd := newTestRootCmd()
	normalizeCmd := *normalizeClarificationCmd
	normalizeCmd.ResetFlags()
	normalizeCmd.Flags().StringVarP(&normalizeQuestion, "question", "q", "", "Question to normalize")
	normalizeCmd.MarkFlagRequired("question")
	cmd.AddCommand(&normalizeCmd)

	cmd.SetArgs([]string{"normalize-clarification"}) // No -q flag

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when -q flag is missing")
	}
}
