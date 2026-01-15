package semantic

import (
	"math"
	"testing"
)

// Helper function to create test SearchResult slices
func makeTestResults(ids ...string) []SearchResult {
	results := make([]SearchResult, len(ids))
	for i, id := range ids {
		results[i] = SearchResult{
			Chunk: Chunk{
				ID:       id,
				Name:     id,
				FilePath: "/test/" + id + ".go",
				Type:     ChunkFunction,
			},
			Score: float32(100 - i*10), // Descending scores: 100, 90, 80, ...
		}
	}
	return results
}

// TestRRFusion_BasicCombination verifies the RRF formula application.
// Given dense results [A(rank 1), B(rank 2), C(rank 3)]
// And lexical results [B(rank 1), D(rank 2), A(rank 3)]
// Then B should rank highest (appears in both with good ranks).
func TestRRFusion_BasicCombination(t *testing.T) {
	denseResults := makeTestResults("A", "B", "C")
	lexicalResults := makeTestResults("B", "D", "A")

	// k=60 default
	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) != 4 { // A, B, C, D
		t.Fatalf("expected 4 unique results, got %d", len(results))
	}

	// B should be first: 1/(60+2) + 1/(60+1) = 1/62 + 1/61 ≈ 0.0325
	// A should be second: 1/(60+1) + 1/(60+3) = 1/61 + 1/63 ≈ 0.0323
	// C: 1/(60+3) only ≈ 0.0159
	// D: 1/(60+2) only ≈ 0.0161
	if results[0].Chunk.ID != "B" {
		t.Errorf("expected B to rank first, got %s", results[0].Chunk.ID)
	}

	if results[1].Chunk.ID != "A" {
		t.Errorf("expected A to rank second, got %s", results[1].Chunk.ID)
	}

	// Verify all unique results are included
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.Chunk.ID] = true
	}
	for _, expected := range []string{"A", "B", "C", "D"} {
		if !ids[expected] {
			t.Errorf("expected result %s to be included", expected)
		}
	}
}

// TestRRFusion_ExactMatchBoost verifies that exact matches rank higher.
func TestRRFusion_ExactMatchBoost(t *testing.T) {
	// Dense search ranks semantically similar result first
	denseResults := makeTestResults("validateAuth", "handleAuthCallback", "processAuth")

	// Lexical search ranks exact match first
	lexicalResults := makeTestResults("handleAuthCallback", "authCallback", "authHandler")

	results := FuseRRF(denseResults, lexicalResults, 60)

	// handleAuthCallback appears in both lists - should rank high
	found := false
	for i, r := range results {
		if r.Chunk.ID == "handleAuthCallback" {
			if i > 2 {
				t.Errorf("expected handleAuthCallback in top 3, got rank %d", i+1)
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("handleAuthCallback should be in results")
	}
}

// TestRRFusion_EmptyDense verifies handling when dense search returns no results.
func TestRRFusion_EmptyDense(t *testing.T) {
	denseResults := []SearchResult{} // empty
	lexicalResults := makeTestResults("A", "B", "C")

	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) != 3 {
		t.Errorf("expected 3 results from lexical-only, got %d", len(results))
	}

	// Results should be from lexical with adjusted RRF scores
	for i, r := range results {
		if r.Score <= 0 {
			t.Errorf("result %d should have positive score, got %f", i, r.Score)
		}
	}

	// Order should be preserved (A, B, C)
	if results[0].Chunk.ID != "A" {
		t.Errorf("expected A first, got %s", results[0].Chunk.ID)
	}
}

// TestRRFusion_EmptyLexical verifies handling when lexical search returns no results.
func TestRRFusion_EmptyLexical(t *testing.T) {
	denseResults := makeTestResults("A", "B", "C")
	lexicalResults := []SearchResult{} // empty

	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) != 3 {
		t.Errorf("expected 3 results from dense-only, got %d", len(results))
	}

	// Results should be from dense with adjusted RRF scores
	for i, r := range results {
		if r.Score <= 0 {
			t.Errorf("result %d should have positive score, got %f", i, r.Score)
		}
	}

	// Order should be preserved
	if results[0].Chunk.ID != "A" {
		t.Errorf("expected A first, got %s", results[0].Chunk.ID)
	}
}

// TestRRFusion_BothEmpty verifies handling when both searches return no results.
func TestRRFusion_BothEmpty(t *testing.T) {
	denseResults := []SearchResult{}
	lexicalResults := []SearchResult{}

	results := FuseRRF(denseResults, lexicalResults, 60)

	if results == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestRRFusion_NilInputs verifies graceful handling of nil slices.
func TestRRFusion_NilInputs(t *testing.T) {
	t.Run("NilDense", func(t *testing.T) {
		results := FuseRRF(nil, makeTestResults("A", "B"), 60)
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("NilLexical", func(t *testing.T) {
		results := FuseRRF(makeTestResults("A", "B"), nil, 60)
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("BothNil", func(t *testing.T) {
		results := FuseRRF(nil, nil, 60)
		if results == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

// TestRRFusion_IdenticalResults verifies handling when both lists have same items.
func TestRRFusion_IdenticalResults(t *testing.T) {
	denseResults := makeTestResults("A", "B", "C")
	lexicalResults := makeTestResults("A", "B", "C")

	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Order should be preserved (A, B, C)
	expected := []string{"A", "B", "C"}
	for i, exp := range expected {
		if results[i].Chunk.ID != exp {
			t.Errorf("position %d: expected %s, got %s", i, exp, results[i].Chunk.ID)
		}
	}

	// Scores should be boosted (sum of contributions from both lists)
	// A: 1/(60+1) + 1/(60+1) = 2/61 ≈ 0.0328
	expectedScore := float32(2.0 / 61.0)
	tolerance := float32(0.0001)
	if math.Abs(float64(results[0].Score-expectedScore)) > float64(tolerance) {
		t.Errorf("expected A score ≈ %f, got %f", expectedScore, results[0].Score)
	}
}

// TestRRFusion_ConfigurableK tests different k values.
func TestRRFusion_ConfigurableK(t *testing.T) {
	denseResults := makeTestResults("A", "B")
	lexicalResults := makeTestResults("B", "A")

	t.Run("DefaultK60", func(t *testing.T) {
		results := FuseRRF(denseResults, lexicalResults, 60)
		// A: 1/61 + 1/62 ≈ 0.0325
		// B: 1/62 + 1/61 ≈ 0.0325
		// Scores should be very close
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		diff := math.Abs(float64(results[0].Score - results[1].Score))
		if diff > 0.001 {
			t.Errorf("with k=60, score difference should be minimal, got %f", diff)
		}
	})

	t.Run("SmallK1", func(t *testing.T) {
		results := FuseRRF(denseResults, lexicalResults, 1)
		// A: 1/2 + 1/3 ≈ 0.833
		// B: 1/3 + 1/2 ≈ 0.833
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// With small k, rank differences matter more
		expectedScore := float32(1.0/2.0 + 1.0/3.0)
		tolerance := float32(0.01)
		if math.Abs(float64(results[0].Score-expectedScore)) > float64(tolerance) {
			t.Errorf("with k=1, expected score ≈ %f, got %f", expectedScore, results[0].Score)
		}
	})

	t.Run("LargeK1000", func(t *testing.T) {
		results := FuseRRF(denseResults, lexicalResults, 1000)
		// With large k, score differences are minimal
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// All scores should be very small but positive
		for _, r := range results {
			if r.Score <= 0 || r.Score > 0.01 {
				t.Errorf("with k=1000, expected small positive score, got %f", r.Score)
			}
		}
	})
}

// TestRRFusion_InvalidK verifies error handling for invalid k parameter.
func TestRRFusion_InvalidK(t *testing.T) {
	denseResults := makeTestResults("A")
	lexicalResults := makeTestResults("B")

	t.Run("NegativeK", func(t *testing.T) {
		results, err := FuseRRFWithError(denseResults, lexicalResults, -1)
		if err == nil {
			t.Error("expected error for negative k")
		}
		if results != nil {
			t.Error("expected nil results on error")
		}
	})

	t.Run("ZeroK", func(t *testing.T) {
		results, err := FuseRRFWithError(denseResults, lexicalResults, 0)
		if err == nil {
			t.Error("expected error for zero k")
		}
		if results != nil {
			t.Error("expected nil results on error")
		}
	})
}

// TestWeightedFusion_Alpha tests weighted alpha parameter for blending.
func TestWeightedFusion_Alpha(t *testing.T) {
	// Dense result with high score
	denseResults := []SearchResult{
		{Chunk: Chunk{ID: "A", Name: "A"}, Score: 0.9},
		{Chunk: Chunk{ID: "B", Name: "B"}, Score: 0.5},
	}
	// Lexical result with different rankings
	lexicalResults := []SearchResult{
		{Chunk: Chunk{ID: "B", Name: "B"}, Score: 0.8},
		{Chunk: Chunk{ID: "C", Name: "C"}, Score: 0.6},
	}

	t.Run("Alpha0.7_DenseHeavy", func(t *testing.T) {
		results, err := FuseWeighted(denseResults, lexicalResults, 0.7)
		if err != nil {
			t.Fatalf("FuseWeighted failed: %v", err)
		}

		// A only in dense: 0.7 * 0.9 = 0.63
		// B in both: 0.7 * 0.5 + 0.3 * 0.8 = 0.35 + 0.24 = 0.59
		// C only in lexical: 0.3 * 0.6 = 0.18
		// So A > B > C
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		if results[0].Chunk.ID != "A" {
			t.Errorf("expected A first with alpha=0.7, got %s", results[0].Chunk.ID)
		}
	})

	t.Run("Alpha0.3_LexicalHeavy", func(t *testing.T) {
		results, err := FuseWeighted(denseResults, lexicalResults, 0.3)
		if err != nil {
			t.Fatalf("FuseWeighted failed: %v", err)
		}

		// A only in dense: 0.3 * 0.9 = 0.27
		// B in both: 0.3 * 0.5 + 0.7 * 0.8 = 0.15 + 0.56 = 0.71
		// C only in lexical: 0.7 * 0.6 = 0.42
		// So B > C > A
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		if results[0].Chunk.ID != "B" {
			t.Errorf("expected B first with alpha=0.3, got %s", results[0].Chunk.ID)
		}
	})

	t.Run("Alpha0.5_Balanced", func(t *testing.T) {
		results, err := FuseWeighted(denseResults, lexicalResults, 0.5)
		if err != nil {
			t.Fatalf("FuseWeighted failed: %v", err)
		}

		// Results should be balanced
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
	})
}

// TestWeightedFusion_InvalidAlpha verifies error handling for invalid alpha.
func TestWeightedFusion_InvalidAlpha(t *testing.T) {
	denseResults := makeTestResults("A")
	lexicalResults := makeTestResults("B")

	t.Run("NegativeAlpha", func(t *testing.T) {
		_, err := FuseWeighted(denseResults, lexicalResults, -0.1)
		if err == nil {
			t.Error("expected error for negative alpha")
		}
	})

	t.Run("AlphaGreaterThan1", func(t *testing.T) {
		_, err := FuseWeighted(denseResults, lexicalResults, 1.1)
		if err == nil {
			t.Error("expected error for alpha > 1")
		}
	})

	t.Run("AlphaExactly0", func(t *testing.T) {
		// Alpha=0 means 100% lexical - should be valid
		results, err := FuseWeighted(denseResults, lexicalResults, 0.0)
		if err != nil {
			t.Errorf("alpha=0 should be valid, got error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results even with alpha=0")
		}
	})

	t.Run("AlphaExactly1", func(t *testing.T) {
		// Alpha=1 means 100% dense - should be valid
		results, err := FuseWeighted(denseResults, lexicalResults, 1.0)
		if err != nil {
			t.Errorf("alpha=1 should be valid, got error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results even with alpha=1")
		}
	})
}

// TestFusion_TopKLimit verifies that TopK is applied after fusion.
func TestFusion_TopKLimit(t *testing.T) {
	// Create 20 dense results
	denseIDs := make([]string, 20)
	for i := 0; i < 20; i++ {
		denseIDs[i] = string(rune('A' + i))
	}
	denseResults := makeTestResults(denseIDs...)

	// Create 15 lexical results (some overlap)
	lexicalIDs := make([]string, 15)
	for i := 0; i < 15; i++ {
		lexicalIDs[i] = string(rune('J' + i)) // J through X (overlaps with dense)
	}
	lexicalResults := makeTestResults(lexicalIDs...)

	// Fuse with TopK=10
	results := FuseRRFWithTopK(denseResults, lexicalResults, 60, 10)

	if len(results) > 10 {
		t.Errorf("expected max 10 results with TopK=10, got %d", len(results))
	}

	if len(results) == 0 {
		t.Error("expected some results")
	}

	// Results should be sorted by fused score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: position %d has higher score than %d", i, i-1)
		}
	}
}

// TestFusion_TopKZero verifies that TopK=0 returns all results.
func TestFusion_TopKZero(t *testing.T) {
	denseResults := makeTestResults("A", "B", "C")
	lexicalResults := makeTestResults("D", "E", "F")

	results := FuseRRFWithTopK(denseResults, lexicalResults, 60, 0)

	if len(results) != 6 {
		t.Errorf("expected all 6 results with TopK=0, got %d", len(results))
	}
}

// TestFusion_ScoreNormalization verifies scores are in valid range.
func TestFusion_ScoreNormalization(t *testing.T) {
	denseResults := makeTestResults("A", "B", "C", "D", "E")
	lexicalResults := makeTestResults("C", "D", "E", "F", "G")

	results := FuseRRF(denseResults, lexicalResults, 60)

	for i, r := range results {
		if r.Score < 0 {
			t.Errorf("result %d has negative score: %f", i, r.Score)
		}
		// RRF scores should be reasonable (sum of 1/(k+rank) terms)
		// Maximum possible: 2 * 1/61 ≈ 0.033 (if result is rank 1 in both lists)
		if r.Score > 1.0 {
			t.Errorf("result %d has unexpectedly high score: %f", i, r.Score)
		}
	}
}

// TestFusion_PreservesChunkData verifies that chunk metadata is preserved.
func TestFusion_PreservesChunkData(t *testing.T) {
	denseResults := []SearchResult{
		{
			Chunk: Chunk{
				ID:        "test-id",
				FilePath:  "/path/to/file.go",
				Type:      ChunkFunction,
				Name:      "TestFunction",
				Signature: "func TestFunction() error",
				Content:   "func TestFunction() error { return nil }",
				StartLine: 10,
				EndLine:   15,
				Language:  "go",
			},
			Score: 0.9,
		},
	}
	lexicalResults := []SearchResult{}

	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Chunk.ID != "test-id" {
		t.Errorf("ID not preserved: got %s", r.Chunk.ID)
	}
	if r.Chunk.FilePath != "/path/to/file.go" {
		t.Errorf("FilePath not preserved: got %s", r.Chunk.FilePath)
	}
	if r.Chunk.Type != ChunkFunction {
		t.Errorf("Type not preserved: got %s", r.Chunk.Type)
	}
	if r.Chunk.Name != "TestFunction" {
		t.Errorf("Name not preserved: got %s", r.Chunk.Name)
	}
	if r.Chunk.Signature != "func TestFunction() error" {
		t.Errorf("Signature not preserved: got %s", r.Chunk.Signature)
	}
	if r.Chunk.StartLine != 10 || r.Chunk.EndLine != 15 {
		t.Errorf("Line numbers not preserved: got %d-%d", r.Chunk.StartLine, r.Chunk.EndLine)
	}
	if r.Chunk.Language != "go" {
		t.Errorf("Language not preserved: got %s", r.Chunk.Language)
	}
}

// TestFusion_LargeResultSets verifies performance with large result sets.
func TestFusion_LargeResultSets(t *testing.T) {
	// Create 1000 dense results
	denseIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		denseIDs[i] = "dense_" + string(rune(i))
	}
	denseResults := make([]SearchResult, 1000)
	for i := 0; i < 1000; i++ {
		denseResults[i] = SearchResult{
			Chunk: Chunk{ID: denseIDs[i], Name: denseIDs[i]},
			Score: float32(1000 - i),
		}
	}

	// Create 1000 lexical results (50% overlap)
	lexicalResults := make([]SearchResult, 1000)
	for i := 0; i < 1000; i++ {
		var id string
		if i < 500 {
			id = "dense_" + string(rune(i*2)) // Overlap with even-indexed dense results
		} else {
			id = "lexical_" + string(rune(i))
		}
		lexicalResults[i] = SearchResult{
			Chunk: Chunk{ID: id, Name: id},
			Score: float32(1000 - i),
		}
	}

	// Should complete without error
	results := FuseRRF(denseResults, lexicalResults, 60)

	if len(results) == 0 {
		t.Error("expected non-empty results")
	}

	// Verify results are sorted
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted at position %d", i)
			break
		}
	}
}

// TestFusion_DuplicateIDsInSameList verifies handling of duplicates within a list.
func TestFusion_DuplicateIDsInSameList(t *testing.T) {
	// Dense results with duplicate ID
	denseResults := []SearchResult{
		{Chunk: Chunk{ID: "A", Name: "A"}, Score: 0.9},
		{Chunk: Chunk{ID: "A", Name: "A"}, Score: 0.5}, // Duplicate
		{Chunk: Chunk{ID: "B", Name: "B"}, Score: 0.3},
	}
	lexicalResults := makeTestResults("C")

	results := FuseRRF(denseResults, lexicalResults, 60)

	// Should handle gracefully - either dedupe or use first occurrence
	idCount := make(map[string]int)
	for _, r := range results {
		idCount[r.Chunk.ID]++
	}

	if idCount["A"] > 1 {
		t.Errorf("duplicate ID 'A' should be deduplicated, found %d occurrences", idCount["A"])
	}
}
