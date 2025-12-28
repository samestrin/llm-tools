package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// YAMLStorage implements Storage interface using YAML file backend.
// Thread-safe via mutex for all operations.
type YAMLStorage struct {
	path   string
	mu     sync.RWMutex
	data   *tracking.TrackingFile
	closed bool
	dirty  bool
}

// NewYAMLStorage creates a new YAML storage instance.
// Creates an empty tracking file if it doesn't exist.
func NewYAMLStorage(ctx context.Context, path string) (*YAMLStorage, error) {
	// Validate path
	if path == "" {
		return nil, ErrInvalidPath
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return nil, &UnsupportedBackendError{Extension: ext}
	}

	y := &YAMLStorage{
		path: path,
	}

	// Load existing file or create new
	if tracking.FileExists(path) {
		tf, err := tracking.LoadTrackingFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load tracking file: %w", err)
		}
		y.data = tf
	} else {
		// Create new tracking file
		y.data = tracking.NewTrackingFile(time.Now().Format("2006-01-02"))
		y.dirty = true
		if err := y.save(); err != nil {
			return nil, fmt.Errorf("failed to create tracking file: %w", err)
		}
	}

	return y, nil
}

// save writes the current data to disk (must hold write lock).
func (y *YAMLStorage) save() error {
	if !y.dirty {
		return nil
	}
	y.data.LastUpdated = time.Now().Format("2006-01-02")
	if err := tracking.SaveTrackingFile(y.data, y.path); err != nil {
		return err
	}
	y.dirty = false
	return nil
}

// findEntry finds an entry by ID (must hold read lock).
func (y *YAMLStorage) findEntry(id string) (*tracking.Entry, int) {
	for i := range y.data.Entries {
		if y.data.Entries[i].ID == id {
			return &y.data.Entries[i], i
		}
	}
	return nil, -1
}

// Create adds a new entry to storage.
func (y *YAMLStorage) Create(ctx context.Context, entry *tracking.Entry) error {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return ErrStorageClosed
	}

	if existing, _ := y.findEntry(entry.ID); existing != nil {
		return &DuplicateEntryError{ID: entry.ID}
	}

	y.data.Entries = append(y.data.Entries, *entry)
	y.dirty = true
	return y.save()
}

// Read retrieves an entry by ID.
func (y *YAMLStorage) Read(ctx context.Context, id string) (*tracking.Entry, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	entry, _ := y.findEntry(id)
	if entry == nil {
		return nil, &NotFoundError{ID: id}
	}

	// Return a copy to avoid data races
	copy := *entry
	return &copy, nil
}

// Update modifies an existing entry.
func (y *YAMLStorage) Update(ctx context.Context, entry *tracking.Entry) error {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return ErrStorageClosed
	}

	_, idx := y.findEntry(entry.ID)
	if idx < 0 {
		return &NotFoundError{ID: entry.ID}
	}

	y.data.Entries[idx] = *entry
	y.dirty = true
	return y.save()
}

// Delete removes an entry by ID.
func (y *YAMLStorage) Delete(ctx context.Context, id string) error {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return ErrStorageClosed
	}

	_, idx := y.findEntry(id)
	if idx < 0 {
		return &NotFoundError{ID: id}
	}

	// Remove entry by swapping with last and truncating
	last := len(y.data.Entries) - 1
	y.data.Entries[idx] = y.data.Entries[last]
	y.data.Entries = y.data.Entries[:last]
	y.dirty = true
	return y.save()
}

// List returns entries matching the filter criteria.
func (y *YAMLStorage) List(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	result := make([]tracking.Entry, 0, len(y.data.Entries))
	for _, entry := range y.data.Entries {
		if y.matchesFilter(&entry, filter) {
			result = append(result, entry)
		}
	}

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []tracking.Entry{}, nil
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// matchesFilter checks if an entry matches the filter criteria.
func (y *YAMLStorage) matchesFilter(entry *tracking.Entry, filter ListFilter) bool {
	// Filter by status
	if filter.Status != "" && entry.Status != filter.Status {
		return false
	}

	// Filter by min occurrences
	if filter.MinOccurrences > 0 && entry.Occurrences < filter.MinOccurrences {
		return false
	}

	// Filter by tags (any match)
	if len(filter.Tags) > 0 {
		found := false
		for _, filterTag := range filter.Tags {
			for _, entryTag := range entry.ContextTags {
				if entryTag == filterTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by sprint
	if filter.Sprint != "" {
		found := false
		for _, sprint := range entry.SprintsSeen {
			if sprint == filter.Sprint {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by query (simple substring search for YAML)
	if filter.Query != "" {
		query := strings.ToLower(filter.Query)
		if !strings.Contains(strings.ToLower(entry.CanonicalQuestion), query) &&
			!strings.Contains(strings.ToLower(entry.CurrentAnswer), query) {
			return false
		}
	}

	return true
}

// FindByQuestion searches for an entry by canonical question.
func (y *YAMLStorage) FindByQuestion(ctx context.Context, question string) (*tracking.Entry, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	for i := range y.data.Entries {
		if y.data.Entries[i].CanonicalQuestion == question {
			copy := y.data.Entries[i]
			return &copy, nil
		}
	}
	return nil, &NotFoundError{ID: question}
}

// GetByTags returns entries containing any of the specified tags.
func (y *YAMLStorage) GetByTags(ctx context.Context, tags []string) ([]tracking.Entry, error) {
	return y.List(ctx, ListFilter{Tags: tags})
}

// GetBySprint returns entries seen in the specified sprint.
func (y *YAMLStorage) GetBySprint(ctx context.Context, sprint string) ([]tracking.Entry, error) {
	return y.List(ctx, ListFilter{Sprint: sprint})
}

// BulkInsert adds multiple entries in a single operation.
func (y *YAMLStorage) BulkInsert(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	result := &BulkResult{}
	for i := range entries {
		result.Processed++
		if existing, _ := y.findEntry(entries[i].ID); existing != nil {
			result.Skipped++
			continue
		}
		y.data.Entries = append(y.data.Entries, entries[i])
		result.Created++
	}

	if result.Created > 0 {
		y.dirty = true
		if err := y.save(); err != nil {
			result.Errors = append(result.Errors, err)
			return result, err
		}
	}

	return result, nil
}

// BulkUpdate modifies multiple entries in a single operation.
func (y *YAMLStorage) BulkUpdate(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	result := &BulkResult{}
	for i := range entries {
		result.Processed++
		_, idx := y.findEntry(entries[i].ID)
		if idx < 0 {
			result.Skipped++
			continue
		}
		y.data.Entries[idx] = entries[i]
		result.Updated++
	}

	if result.Updated > 0 {
		y.dirty = true
		if err := y.save(); err != nil {
			result.Errors = append(result.Errors, err)
			return result, err
		}
	}

	return result, nil
}

// BulkDelete removes multiple entries by ID.
func (y *YAMLStorage) BulkDelete(ctx context.Context, ids []string) (*BulkResult, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	result := &BulkResult{}
	// Build set of IDs to delete
	deleteSet := make(map[string]bool)
	for _, id := range ids {
		deleteSet[id] = true
	}

	// Filter entries, keeping only those not in delete set
	newEntries := make([]tracking.Entry, 0, len(y.data.Entries))
	for _, entry := range y.data.Entries {
		if deleteSet[entry.ID] {
			result.Processed++
			delete(deleteSet, entry.ID)
		} else {
			newEntries = append(newEntries, entry)
		}
	}
	result.Skipped = len(deleteSet) // Remaining are not found

	if result.Processed > 0 {
		y.data.Entries = newEntries
		y.dirty = true
		if err := y.save(); err != nil {
			result.Errors = append(result.Errors, err)
			return result, err
		}
	}

	return result, nil
}

// Import loads entries with specified mode.
func (y *YAMLStorage) Import(ctx context.Context, entries []tracking.Entry, mode ImportMode) (*BulkResult, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	result := &BulkResult{}

	switch mode {
	case ImportModeOverwrite:
		y.data.Entries = make([]tracking.Entry, len(entries))
		copy(y.data.Entries, entries)
		result.Created = len(entries)
		result.Processed = len(entries)
	case ImportModeMerge:
		for i := range entries {
			result.Processed++
			if _, idx := y.findEntry(entries[i].ID); idx >= 0 {
				y.data.Entries[idx] = entries[i]
				result.Updated++
			} else {
				y.data.Entries = append(y.data.Entries, entries[i])
				result.Created++
			}
		}
	case ImportModeAppend:
		fallthrough
	default:
		for i := range entries {
			result.Processed++
			if _, idx := y.findEntry(entries[i].ID); idx >= 0 {
				result.Skipped++
			} else {
				y.data.Entries = append(y.data.Entries, entries[i])
				result.Created++
			}
		}
	}

	y.dirty = true
	if err := y.save(); err != nil {
		result.Errors = append(result.Errors, err)
		return result, err
	}

	return result, nil
}

// Export returns all entries matching the filter.
func (y *YAMLStorage) Export(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	return y.List(ctx, filter)
}

// Vacuum is a no-op for YAML storage.
func (y *YAMLStorage) Vacuum(ctx context.Context) (int64, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return 0, ErrStorageClosed
	}

	// YAML doesn't need vacuum, but we can get file info
	info, err := os.Stat(y.path)
	if err != nil {
		return 0, nil
	}
	return info.Size(), nil
}

// Stats returns storage statistics.
func (y *YAMLStorage) Stats(ctx context.Context) (*StorageStats, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()

	if y.closed {
		return nil, ErrStorageClosed
	}

	stats := &StorageStats{
		TotalEntries:    len(y.data.Entries),
		EntriesByStatus: make(map[string]int),
		LastModified:    y.data.LastUpdated,
	}

	uniqueTags := make(map[string]bool)
	uniqueSprints := make(map[string]bool)

	for _, entry := range y.data.Entries {
		stats.EntriesByStatus[entry.Status]++
		stats.TotalVariants += len(entry.Variants)
		for _, tag := range entry.ContextTags {
			uniqueTags[tag] = true
		}
		for _, sprint := range entry.SprintsSeen {
			uniqueSprints[sprint] = true
		}
	}

	stats.TotalTags = len(uniqueTags)
	stats.TotalSprints = len(uniqueSprints)

	// Get file size
	if info, err := os.Stat(y.path); err == nil {
		stats.StorageSize = info.Size()
	}

	return stats, nil
}

// Backup copies the YAML file to the specified path.
func (y *YAMLStorage) Backup(ctx context.Context, path string) error {
	y.mu.RLock()
	defer y.mu.RUnlock()

	if y.closed {
		return ErrStorageClosed
	}

	// Read current file
	data, err := os.ReadFile(y.path)
	if err != nil {
		return fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Write backup
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// Close releases resources.
func (y *YAMLStorage) Close() error {
	y.mu.Lock()
	defer y.mu.Unlock()

	if y.closed {
		return nil
	}

	// Save any pending changes
	if err := y.save(); err != nil {
		return err
	}

	y.closed = true
	return nil
}

// Verify YAMLStorage implements Storage interface
var _ Storage = (*YAMLStorage)(nil)
