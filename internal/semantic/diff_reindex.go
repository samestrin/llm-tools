package semantic

// ChunkSummary is a lightweight representation of a stored chunk,
// used for diff-based re-indexing without loading full content or embeddings.
type ChunkSummary struct {
	ID          string
	ContentHash string
	Name        string
	StartLine   int
}

// DiffResult describes the outcome of comparing old vs new chunks for a file.
type DiffResult struct {
	// Reuse contains new chunks whose content matches an old chunk.
	// Key: new chunk index, Value: old chunk ID whose embedding should be copied.
	Reuse map[int]string

	// NeedEmbed contains indices into the new chunks slice that need fresh embeddings.
	NeedEmbed []int

	// Delete contains old chunk IDs that are no longer present in the new chunks.
	Delete []string
}

// DiffChunks compares old chunk summaries (from storage) against new chunks (from re-chunking)
// and determines which chunks can reuse existing embeddings, which need new embeddings,
// and which old chunks should be deleted.
//
// Matching is done by content hash. If an old chunk has the same content hash as a new chunk,
// the old embedding is reused (even if the chunk ID changed, e.g., due to a line number shift).
// Old chunks with empty ContentHash (legacy rows) are treated as non-matching.
func DiffChunks(oldSummaries []ChunkSummary, newChunks []Chunk) DiffResult {
	result := DiffResult{
		Reuse: make(map[int]string),
	}

	// Build map from content hash → old chunk ID (first match wins)
	oldByHash := make(map[string]string, len(oldSummaries))
	oldConsumed := make(map[string]bool, len(oldSummaries))
	for _, old := range oldSummaries {
		if old.ContentHash != "" {
			if _, exists := oldByHash[old.ContentHash]; !exists {
				oldByHash[old.ContentHash] = old.ID
			}
		}
	}

	// Classify each new chunk
	for i, newChunk := range newChunks {
		hash := newChunk.ContentHash
		if hash == "" {
			hash = newChunk.ComputeContentHash()
		}

		if oldID, ok := oldByHash[hash]; ok && !oldConsumed[oldID] {
			// Content matches an old chunk — reuse its embedding
			result.Reuse[i] = oldID
			oldConsumed[oldID] = true
		} else {
			// New or changed content — needs embedding
			result.NeedEmbed = append(result.NeedEmbed, i)
		}
	}

	// Find old chunks not consumed — they were removed
	for _, old := range oldSummaries {
		if !oldConsumed[old.ID] {
			result.Delete = append(result.Delete, old.ID)
		}
	}

	return result
}
