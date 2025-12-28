package storage

import (
	"context"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// Storage defines the interface for clarification entry persistence.
// Implementations must be safe for concurrent use from multiple goroutines.
type Storage interface {
	// CRUD Operations

	// Create adds a new entry to storage.
	// Returns ErrDuplicateEntry if an entry with the same ID exists.
	Create(ctx context.Context, entry *tracking.Entry) error

	// Read retrieves an entry by ID.
	// Returns ErrNotFound if the entry does not exist.
	Read(ctx context.Context, id string) (*tracking.Entry, error)

	// Update modifies an existing entry.
	// Returns ErrNotFound if the entry does not exist.
	// Returns ErrConcurrentModification if the entry was modified since last read.
	Update(ctx context.Context, entry *tracking.Entry) error

	// Delete removes an entry by ID.
	// Returns ErrNotFound if the entry does not exist.
	Delete(ctx context.Context, id string) error

	// Query Operations

	// List returns entries matching the filter criteria.
	// Returns an empty slice (not nil) if no entries match.
	List(ctx context.Context, filter ListFilter) ([]tracking.Entry, error)

	// FindByQuestion searches for an entry by canonical question.
	// Returns ErrNotFound if no matching entry exists.
	FindByQuestion(ctx context.Context, question string) (*tracking.Entry, error)

	// GetByTags returns entries containing any of the specified tags.
	GetByTags(ctx context.Context, tags []string) ([]tracking.Entry, error)

	// GetBySprint returns entries seen in the specified sprint.
	GetBySprint(ctx context.Context, sprint string) ([]tracking.Entry, error)

	// Bulk Operations

	// BulkInsert adds multiple entries in a single transaction.
	// Returns a BulkResult with counts of created/updated/skipped entries.
	BulkInsert(ctx context.Context, entries []tracking.Entry) (*BulkResult, error)

	// BulkUpdate modifies multiple entries in a single transaction.
	BulkUpdate(ctx context.Context, entries []tracking.Entry) (*BulkResult, error)

	// BulkDelete removes multiple entries by ID in a single transaction.
	BulkDelete(ctx context.Context, ids []string) (*BulkResult, error)

	// Import loads entries with specified mode (append/overwrite/merge).
	Import(ctx context.Context, entries []tracking.Entry, mode ImportMode) (*BulkResult, error)

	// Export returns all entries matching the filter.
	Export(ctx context.Context, filter ListFilter) ([]tracking.Entry, error)

	// Maintenance Operations

	// Vacuum performs storage optimization (e.g., SQLite VACUUM).
	// Returns the number of bytes reclaimed.
	Vacuum(ctx context.Context) (int64, error)

	// Stats returns storage statistics.
	Stats(ctx context.Context) (*StorageStats, error)

	// Backup creates a backup of the storage at the specified path.
	Backup(ctx context.Context, path string) error

	// Lifecycle

	// Close releases storage resources.
	// After Close, all operations return ErrStorageClosed.
	Close() error
}
