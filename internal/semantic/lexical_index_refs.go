package semantic

import "context"

// LexicalIndex stores the call graph as well as the lexical index, so that a
// vector backend which cannot express reference joins can delegate them here.
var _ RefStorage = (*LexicalIndex)(nil)

// StoreRefs stores chunk references in a single transaction.
func (idx *LexicalIndex) StoreRefs(ctx context.Context, refs []ChunkRef) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	return storeRefsSQL(ctx, idx.db, refs)
}

// GetRefs retrieves all references leaving a chunk.
func (idx *LexicalIndex) GetRefs(ctx context.Context, chunkID string) ([]ChunkRef, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrStorageClosed
	}

	return getRefsSQL(ctx, idx.db, chunkID)
}

// GetCallers retrieves all references arriving at a chunk.
func (idx *LexicalIndex) GetCallers(ctx context.Context, chunkID string) ([]ChunkRef, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrStorageClosed
	}

	return getCallersSQL(ctx, idx.db, chunkID)
}

// DeleteRefsByChunk removes all references leaving a chunk.
func (idx *LexicalIndex) DeleteRefsByChunk(ctx context.Context, chunkID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	return deleteRefsByChunkSQL(ctx, idx.db, chunkID)
}

// ResolveRefs links stored references to the chunks they name.
func (idx *LexicalIndex) ResolveRefs(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	return resolveRefsSQL(ctx, idx.db)
}

// GetCallersByName returns the call edges arriving at any chunk with this name.
func (idx *LexicalIndex) GetCallersByName(ctx context.Context, name string) ([]RefEdge, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrStorageClosed
	}

	return getCallersByNameSQL(ctx, idx.db, name)
}

// GetRefsByName returns the edges leaving any chunk with this name.
func (idx *LexicalIndex) GetRefsByName(ctx context.Context, name string) ([]RefEdge, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrStorageClosed
	}

	return getRefsByNameSQL(ctx, idx.db, name)
}
