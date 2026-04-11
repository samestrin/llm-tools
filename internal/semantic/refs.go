package semantic

import "context"

// RefType represents the kind of reference between chunks.
type RefType string

const (
	RefCalls      RefType = "calls"
	RefImports    RefType = "imports"
	RefUsesType   RefType = "uses_type"
	RefImplements RefType = "implements"
)

// ChunkRef represents a reference from one chunk to another symbol.
type ChunkRef struct {
	ChunkID     string  `json:"chunk_id"`
	RefType     RefType `json:"ref_type"`
	RefName     string  `json:"ref_name"`                // symbol name (e.g., "fmt.Println", "UserService")
	RefTargetID string  `json:"ref_target_id,omitempty"` // resolved chunk ID (nullable, resolved post-index)
}

// RefExtractor is an optional interface that chunkers can implement
// to extract dependency/call references during chunking.
type RefExtractor interface {
	// ExtractRefs extracts references from previously-generated chunks.
	// Called after Chunk() with the same content.
	ExtractRefs(path string, content []byte, chunks []Chunk) ([]ChunkRef, error)
}

// RefStorage is an optional interface for storage backends that support
// chunk reference tracking. Backends implementing this interface can store
// and query call graph edges.
type RefStorage interface {
	// StoreRefs stores chunk references in a single transaction.
	StoreRefs(ctx context.Context, refs []ChunkRef) error

	// GetRefs retrieves all references FROM a chunk (outgoing edges).
	GetRefs(ctx context.Context, chunkID string) ([]ChunkRef, error)

	// GetCallers retrieves all references TO a chunk (incoming edges).
	GetCallers(ctx context.Context, chunkID string) ([]ChunkRef, error)

	// DeleteRefsByChunk removes all references from a chunk.
	DeleteRefsByChunk(ctx context.Context, chunkID string) error

	// ResolveRefs batch-resolves ref_name to ref_target_id by matching
	// against chunk names in the database.
	ResolveRefs(ctx context.Context) error
}
