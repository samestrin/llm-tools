package semantic

import "fmt"

// EnrichForEmbedding prepends structured metadata to chunk content for better embeddings.
// The format is: [language:type] signature in filepath\n\ncontent
// This enriches the embedding with context about what the code is and where it lives,
// without modifying the stored Content field.
func EnrichForEmbedding(chunk Chunk) string {
	prefix := buildEmbeddingPrefix(chunk)
	content := chunk.Content
	if chunk.EmbedText != "" {
		content = chunk.EmbedText
	}
	return prefix + "\n\n" + content
}

// buildEmbeddingPrefix constructs the metadata prefix for embedding enrichment.
func buildEmbeddingPrefix(chunk Chunk) string {
	// Build type tag: [language:type] or [type] if no language
	var tag string
	if chunk.Language != "" {
		tag = fmt.Sprintf("[%s:%s]", chunk.Language, chunk.Type.String())
	} else {
		tag = fmt.Sprintf("[%s]", chunk.Type.String())
	}

	// Build descriptor: signature (preferred) or name (fallback)
	descriptor := chunk.Signature
	if descriptor == "" {
		descriptor = chunk.Name
	}

	// Build location: "in filepath" for non-file chunks, just filepath for file chunks
	if chunk.Type == ChunkFile {
		return fmt.Sprintf("%s %s", tag, chunk.FilePath)
	}

	return fmt.Sprintf("%s %s in %s", tag, descriptor, chunk.FilePath)
}
