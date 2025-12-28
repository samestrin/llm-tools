package storage

// ImportMode specifies how entries should be imported.
type ImportMode int

const (
	// ImportModeAppend adds new entries without modifying existing ones.
	ImportModeAppend ImportMode = iota
	// ImportModeOverwrite replaces all existing entries with imported ones.
	ImportModeOverwrite
	// ImportModeMerge combines entries, updating existing and adding new.
	ImportModeMerge
)

// String returns the string representation of ImportMode.
func (m ImportMode) String() string {
	switch m {
	case ImportModeAppend:
		return "append"
	case ImportModeOverwrite:
		return "overwrite"
	case ImportModeMerge:
		return "merge"
	default:
		return "unknown"
	}
}

// ParseImportMode converts a string to ImportMode.
func ParseImportMode(s string) ImportMode {
	switch s {
	case "append":
		return ImportModeAppend
	case "overwrite":
		return ImportModeOverwrite
	case "merge":
		return ImportModeMerge
	default:
		return ImportModeAppend
	}
}

// ListFilter specifies criteria for filtering entries.
type ListFilter struct {
	// Status filters by entry status (pending, promoted, expired, rejected).
	Status string

	// MinOccurrences filters entries with at least this many occurrences.
	MinOccurrences int

	// Tags filters entries containing any of these tags.
	Tags []string

	// Sprint filters entries seen in this sprint.
	Sprint string

	// Query performs full-text search on question and answer.
	Query string

	// Offset for pagination (0-based).
	Offset int

	// Limit maximum number of results (0 = no limit).
	Limit int
}

// StorageStats provides statistics about the storage.
type StorageStats struct {
	// TotalEntries is the count of all entries.
	TotalEntries int

	// EntriesByStatus maps status to count.
	EntriesByStatus map[string]int

	// TotalVariants is the count of all variant questions.
	TotalVariants int

	// TotalTags is the count of unique tags.
	TotalTags int

	// TotalSprints is the count of unique sprints.
	TotalSprints int

	// StorageSize is the size in bytes (for SQLite).
	StorageSize int64

	// LastModified is the last modification timestamp.
	LastModified string
}

// BulkResult contains results from a bulk operation.
type BulkResult struct {
	// Processed is the number of entries processed.
	Processed int

	// Created is the number of entries created.
	Created int

	// Updated is the number of entries updated.
	Updated int

	// Skipped is the number of entries skipped.
	Skipped int

	// Errors contains any errors that occurred.
	Errors []error
}
