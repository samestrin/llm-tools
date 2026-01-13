package semantic

import (
	"encoding/json"
	"testing"
)

func TestNewMemoryEntry(t *testing.T) {
	entry := NewMemoryEntry("How should auth tokens be handled?", "Use JWT with 24h expiry")

	if entry.Question != "How should auth tokens be handled?" {
		t.Errorf("Expected question to be set, got %q", entry.Question)
	}
	if entry.Answer != "Use JWT with 24h expiry" {
		t.Errorf("Expected answer to be set, got %q", entry.Answer)
	}
	if entry.Status != MemoryStatusPending {
		t.Errorf("Expected status to be pending, got %q", entry.Status)
	}
	if entry.Source != "manual" {
		t.Errorf("Expected source to be 'manual', got %q", entry.Source)
	}
	if entry.Occurrences != 1 {
		t.Errorf("Expected occurrences to be 1, got %d", entry.Occurrences)
	}
	if entry.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if entry.CreatedAt == "" {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestMemoryEntry_GenerateID(t *testing.T) {
	entry := &MemoryEntry{
		Question:  "Test question",
		CreatedAt: "2026-01-13T12:00:00Z",
	}

	id1 := entry.GenerateID()
	id2 := entry.GenerateID()

	// ID should be deterministic
	if id1 != id2 {
		t.Errorf("GenerateID should be deterministic: %q != %q", id1, id2)
	}

	// ID should start with "mem-" prefix
	if len(id1) < 4 || id1[:4] != "mem-" {
		t.Errorf("ID should start with 'mem-' prefix, got %q", id1)
	}

	// Different content should produce different IDs
	entry2 := &MemoryEntry{
		Question:  "Different question",
		CreatedAt: "2026-01-13T12:00:00Z",
	}
	id3 := entry2.GenerateID()
	if id1 == id3 {
		t.Error("Different questions should produce different IDs")
	}
}

func TestMemoryEntry_EmbeddingText(t *testing.T) {
	entry := &MemoryEntry{
		Question: "How should auth tokens be handled?",
		Answer:   "Use JWT with 24h expiry",
	}

	expected := "Question: How should auth tokens be handled?\nAnswer: Use JWT with 24h expiry"
	actual := entry.EmbeddingText()

	if actual != expected {
		t.Errorf("EmbeddingText mismatch:\nExpected: %q\nActual: %q", expected, actual)
	}
}

func TestMemoryEntry_JSONMarshal(t *testing.T) {
	entry := NewMemoryEntry("Test question", "Test answer")
	entry.Tags = []string{"auth", "security"}
	entry.Source = "sprint-8.6"

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal MemoryEntry: %v", err)
	}

	// Unmarshal back
	var parsed MemoryEntry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal MemoryEntry: %v", err)
	}

	if parsed.Question != entry.Question {
		t.Errorf("Question mismatch: %q != %q", parsed.Question, entry.Question)
	}
	if parsed.Answer != entry.Answer {
		t.Errorf("Answer mismatch: %q != %q", parsed.Answer, entry.Answer)
	}
	if parsed.Status != entry.Status {
		t.Errorf("Status mismatch: %q != %q", parsed.Status, entry.Status)
	}
	if len(parsed.Tags) != len(entry.Tags) {
		t.Errorf("Tags length mismatch: %d != %d", len(parsed.Tags), len(entry.Tags))
	}
}

func TestMemorySearchResult_MinimalJSON(t *testing.T) {
	entry := MemoryEntry{
		ID:       "mem-12345678",
		Question: "How should auth tokens be handled?",
		Answer:   "Use JWT with 24h expiry",
	}

	result := MemorySearchResult{
		Entry: entry,
		Score: 0.95,
	}

	minimal := result.MinimalJSON()

	// Parse the JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(minimal), &parsed); err != nil {
		t.Fatalf("MinimalJSON should produce valid JSON: %v", err)
	}

	// Check fields
	if parsed["id"] != "mem-12345678" {
		t.Errorf("Expected id 'mem-12345678', got %v", parsed["id"])
	}
	if _, ok := parsed["q"]; !ok {
		t.Error("Expected 'q' field in minimal output")
	}
	if parsed["s"].(float64) < 0.94 || parsed["s"].(float64) > 0.96 {
		t.Errorf("Expected score ~0.95, got %v", parsed["s"])
	}
}

func TestMemorySearchResult_MinimalJSON_Truncation(t *testing.T) {
	longQuestion := "This is a very long question that exceeds fifty characters and should be truncated"
	entry := MemoryEntry{
		ID:       "mem-12345678",
		Question: longQuestion,
	}

	result := MemorySearchResult{
		Entry: entry,
		Score: 0.9,
	}

	minimal := result.MinimalJSON()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(minimal), &parsed); err != nil {
		t.Fatalf("MinimalJSON should produce valid JSON: %v", err)
	}

	q := parsed["q"].(string)
	if len(q) > 53 { // 50 chars + "..."
		t.Errorf("Question should be truncated, got length %d: %q", len(q), q)
	}
	if q[len(q)-3:] != "..." {
		t.Errorf("Truncated question should end with '...', got %q", q)
	}
}

func TestMemoryStatus_Values(t *testing.T) {
	// Verify the two status values
	if MemoryStatusPending != "pending" {
		t.Errorf("MemoryStatusPending should be 'pending', got %q", MemoryStatusPending)
	}
	if MemoryStatusPromoted != "promoted" {
		t.Errorf("MemoryStatusPromoted should be 'promoted', got %q", MemoryStatusPromoted)
	}
}
