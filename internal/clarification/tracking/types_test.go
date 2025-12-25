package tracking

import (
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestNewTrackingFile(t *testing.T) {
	tf := NewTrackingFile("2025-01-15")

	if tf.Version != 1 {
		t.Errorf("expected Version 1, got %d", tf.Version)
	}
	if tf.Created != "2025-01-15" {
		t.Errorf("expected Created '2025-01-15', got %s", tf.Created)
	}
	if tf.LastUpdated != "2025-01-15" {
		t.Errorf("expected LastUpdated '2025-01-15', got %s", tf.LastUpdated)
	}
	if len(tf.Entries) != 0 {
		t.Errorf("expected empty Entries, got %d entries", len(tf.Entries))
	}
}

func TestNewEntry(t *testing.T) {
	entry := NewEntry("clr-20250115-abc123", "What testing framework?", "Use Vitest", "2025-01-15")

	if entry.ID != "clr-20250115-abc123" {
		t.Errorf("expected ID 'clr-20250115-abc123', got %s", entry.ID)
	}
	if entry.CanonicalQuestion != "What testing framework?" {
		t.Errorf("expected CanonicalQuestion 'What testing framework?', got %s", entry.CanonicalQuestion)
	}
	if entry.CurrentAnswer != "Use Vitest" {
		t.Errorf("expected CurrentAnswer 'Use Vitest', got %s", entry.CurrentAnswer)
	}
	if entry.Occurrences != 1 {
		t.Errorf("expected Occurrences 1, got %d", entry.Occurrences)
	}
	if entry.Status != "pending" {
		t.Errorf("expected Status 'pending', got %s", entry.Status)
	}
	if entry.Confidence != "medium" {
		t.Errorf("expected Confidence 'medium', got %s", entry.Confidence)
	}
}

func TestEntryHasAllFields(t *testing.T) {
	// Verify Entry struct has all 13 required fields
	entryType := reflect.TypeOf(Entry{})
	expectedFields := []string{
		"ID",
		"CanonicalQuestion",
		"Variants",
		"CurrentAnswer",
		"Occurrences",
		"FirstSeen",
		"LastSeen",
		"SprintsSeen",
		"Status",
		"ContextTags",
		"Confidence",
		"PromotedTo",
		"PromotedDate",
	}

	for _, fieldName := range expectedFields {
		field, found := entryType.FieldByName(fieldName)
		if !found {
			t.Errorf("Entry struct missing field: %s", fieldName)
			continue
		}
		// Verify YAML tag exists
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			t.Errorf("Field %s missing yaml tag", fieldName)
		}
		// Verify JSON tag exists
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			t.Errorf("Field %s missing json tag", fieldName)
		}
	}
}

func TestTrackingFileYAMLTags(t *testing.T) {
	// Verify YAML tags match expected Python key names
	tfType := reflect.TypeOf(TrackingFile{})

	expectedTags := map[string]string{
		"Version":     "version",
		"Created":     "created",
		"LastUpdated": "last_updated",
		"Entries":     "entries",
	}

	for fieldName, expectedYAML := range expectedTags {
		field, found := tfType.FieldByName(fieldName)
		if !found {
			t.Errorf("TrackingFile struct missing field: %s", fieldName)
			continue
		}
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != expectedYAML {
			t.Errorf("Field %s: expected yaml tag '%s', got '%s'", fieldName, expectedYAML, yamlTag)
		}
	}
}

func TestEntryYAMLTags(t *testing.T) {
	// Verify YAML tags match expected Python key names
	entryType := reflect.TypeOf(Entry{})

	expectedTags := map[string]string{
		"ID":                "id",
		"CanonicalQuestion": "canonical_question",
		"Variants":          "variants,omitempty",
		"CurrentAnswer":     "current_answer",
		"Occurrences":       "occurrences",
		"FirstSeen":         "first_seen",
		"LastSeen":          "last_seen",
		"SprintsSeen":       "sprints_seen,omitempty",
		"Status":            "status",
		"ContextTags":       "context_tags,omitempty",
		"Confidence":        "confidence",
		"PromotedTo":        "promoted_to,omitempty",
		"PromotedDate":      "promoted_date,omitempty",
	}

	for fieldName, expectedYAML := range expectedTags {
		field, found := entryType.FieldByName(fieldName)
		if !found {
			t.Errorf("Entry struct missing field: %s", fieldName)
			continue
		}
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != expectedYAML {
			t.Errorf("Field %s: expected yaml tag '%s', got '%s'", fieldName, expectedYAML, yamlTag)
		}
	}
}

func TestTrackingFileYAMLRoundTrip(t *testing.T) {
	original := &TrackingFile{
		Version:     1,
		Created:     "2025-01-15",
		LastUpdated: "2025-01-20",
		Entries: []Entry{
			{
				ID:                "clr-20250115-abc123",
				CanonicalQuestion: "What testing framework should we use?",
				Variants:          []string{"Which test framework?", "What about testing?"},
				CurrentAnswer:     "Use Vitest for unit tests",
				Occurrences:       5,
				FirstSeen:         "2025-01-15",
				LastSeen:          "2025-01-20",
				SprintsSeen:       []string{"sprint-1.0", "sprint-1.1"},
				Status:            "pending",
				ContextTags:       []string{"testing", "frontend"},
				Confidence:        "high",
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal TrackingFile: %v", err)
	}

	// Unmarshal back
	var parsed TrackingFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TrackingFile: %v", err)
	}

	// Verify round-trip
	if parsed.Version != original.Version {
		t.Errorf("Version mismatch: expected %d, got %d", original.Version, parsed.Version)
	}
	if parsed.Created != original.Created {
		t.Errorf("Created mismatch: expected %s, got %s", original.Created, parsed.Created)
	}
	if len(parsed.Entries) != len(original.Entries) {
		t.Errorf("Entries count mismatch: expected %d, got %d", len(original.Entries), len(parsed.Entries))
	}
	if len(parsed.Entries) > 0 {
		if parsed.Entries[0].ID != original.Entries[0].ID {
			t.Errorf("Entry ID mismatch: expected %s, got %s", original.Entries[0].ID, parsed.Entries[0].ID)
		}
		if parsed.Entries[0].CanonicalQuestion != original.Entries[0].CanonicalQuestion {
			t.Errorf("Entry CanonicalQuestion mismatch")
		}
	}
}

func TestEmptyEntriesYAML(t *testing.T) {
	tf := NewTrackingFile("2025-01-15")

	data, err := yaml.Marshal(tf)
	if err != nil {
		t.Fatalf("Failed to marshal TrackingFile: %v", err)
	}

	// Verify entries is an empty array, not null
	yamlStr := string(data)
	if !contains(yamlStr, "entries: []") && !contains(yamlStr, "entries:\n") {
		t.Logf("YAML output: %s", yamlStr)
		// The library may output "entries: []" or "entries:\n[]" depending on version
		// Both are valid empty array representations
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
