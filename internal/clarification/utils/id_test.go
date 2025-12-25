package utils

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateID_Format(t *testing.T) {
	id := GenerateID("test question")

	// ID should match format: clr-YYYYMMDD-xxxxxx
	pattern := `^clr-\d{8}-[a-f0-9]{6}$`
	matched, err := regexp.MatchString(pattern, id)
	if err != nil {
		t.Fatalf("regexp error: %v", err)
	}
	if !matched {
		t.Errorf("ID %q does not match expected format clr-YYYYMMDD-xxxxxx", id)
	}
}

func TestGenerateID_DatePortion(t *testing.T) {
	id := GenerateID("test question")

	// Extract date portion and verify it's today's date
	if len(id) < 16 {
		t.Fatalf("ID too short: %s", id)
	}

	datePortion := id[4:12] // Skip "clr-" and get 8 characters
	today := time.Now().Format("20060102")

	if datePortion != today {
		t.Errorf("Date portion %q does not match today %q", datePortion, today)
	}
}

func TestGenerateID_HashDerivedFromQuestion(t *testing.T) {
	question := "What testing framework should we use?"

	// Generate ID multiple times with same question
	id1 := GenerateID(question)
	id2 := GenerateID(question)

	// Hash portion should be identical for same question (same date)
	hash1 := id1[13:] // After "clr-YYYYMMDD-"
	hash2 := id2[13:]

	if hash1 != hash2 {
		t.Errorf("Hash portions differ for same question: %q vs %q", hash1, hash2)
	}
}

func TestGenerateID_DifferentQuestionsProduceDifferentHashes(t *testing.T) {
	id1 := GenerateID("Question one?")
	id2 := GenerateID("Question two?")

	hash1 := id1[13:]
	hash2 := id2[13:]

	if hash1 == hash2 {
		t.Error("Different questions should produce different hash portions")
	}
}

func TestGenerateID_HashLength(t *testing.T) {
	id := GenerateID("test question")

	// ID format is clr-YYYYMMDD-xxxxxx (4 + 8 + 1 + 6 = 19 characters)
	if len(id) != 19 {
		t.Errorf("Expected ID length 19, got %d: %s", len(id), id)
	}

	// Hash portion should be exactly 6 characters
	hash := id[13:]
	if len(hash) != 6 {
		t.Errorf("Expected hash length 6, got %d: %s", len(hash), hash)
	}
}

func TestGenerateID_EmptyQuestion(t *testing.T) {
	id := GenerateID("")

	// Should still generate valid ID even with empty question
	pattern := `^clr-\d{8}-[a-f0-9]{6}$`
	matched, _ := regexp.MatchString(pattern, id)
	if !matched {
		t.Errorf("Empty question should still produce valid ID format: %s", id)
	}
}

func TestGenerateID_SpecialCharacters(t *testing.T) {
	// Questions with special characters should work
	questions := []string{
		"What about 日本語?",
		"Price: $100.00?",
		"Line1\nLine2?",
		"Tab\there?",
		"Quote \"this\"?",
	}

	pattern := `^clr-\d{8}-[a-f0-9]{6}$`
	for _, q := range questions {
		id := GenerateID(q)
		matched, _ := regexp.MatchString(pattern, id)
		if !matched {
			t.Errorf("Special character question %q produced invalid ID: %s", q, id)
		}
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	// Generate many IDs with different questions
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		question := string(rune('A'+i%26)) + string(rune('a'+i/26))
		id := GenerateID(question)
		if seen[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}
