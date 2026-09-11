package semantic

import "context"

// Qdrant has no way to express the join a reference query needs: resolving a
// symbol name to a chunk and then finding every edge pointing at it is
// relational work, not vector work. References therefore live in the same
// parallel SQLite index that already backs lexical search, which mirrors every
// chunk's id, name and location as it is written.
var _ RefStorage = (*QdrantStorage)(nil)

// StoreRefs stores chunk references in the parallel index.
func (s *QdrantStorage) StoreRefs(ctx context.Context, refs []ChunkRef) error {
	return s.parallelFTS.StoreRefs(ctx, refs)
}

// GetRefs retrieves all references leaving a chunk.
func (s *QdrantStorage) GetRefs(ctx context.Context, chunkID string) ([]ChunkRef, error) {
	return s.parallelFTS.GetRefs(ctx, chunkID)
}

// GetCallers retrieves all references arriving at a chunk.
func (s *QdrantStorage) GetCallers(ctx context.Context, chunkID string) ([]ChunkRef, error) {
	return s.parallelFTS.GetCallers(ctx, chunkID)
}

// DeleteRefsByChunk removes all references leaving a chunk.
func (s *QdrantStorage) DeleteRefsByChunk(ctx context.Context, chunkID string) error {
	return s.parallelFTS.DeleteRefsByChunk(ctx, chunkID)
}

// ResolveRefs links stored references to the chunks they name.
func (s *QdrantStorage) ResolveRefs(ctx context.Context) error {
	return s.parallelFTS.ResolveRefs(ctx)
}

// GetCallersByName returns the call edges arriving at any chunk with this name.
func (s *QdrantStorage) GetCallersByName(ctx context.Context, name string) ([]RefEdge, error) {
	return s.parallelFTS.GetCallersByName(ctx, name)
}

// GetRefsByName returns the edges leaving any chunk with this name.
func (s *QdrantStorage) GetRefsByName(ctx context.Context, name string) ([]RefEdge, error) {
	return s.parallelFTS.GetRefsByName(ctx, name)
}
