package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// MockStorage is a test implementation of the Storage interface.
type MockStorage struct {
	entries map[string]*tracking.Entry
	closed  bool
}

// NewMockStorage creates a new mock storage for testing.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		entries: make(map[string]*tracking.Entry),
	}
}

func (m *MockStorage) Create(ctx context.Context, entry *tracking.Entry) error {
	if m.closed {
		return ErrStorageClosed
	}
	if _, exists := m.entries[entry.ID]; exists {
		return &DuplicateEntryError{ID: entry.ID}
	}
	m.entries[entry.ID] = entry
	return nil
}

func (m *MockStorage) Read(ctx context.Context, id string) (*tracking.Entry, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	entry, exists := m.entries[id]
	if !exists {
		return nil, &NotFoundError{ID: id}
	}
	return entry, nil
}

func (m *MockStorage) Update(ctx context.Context, entry *tracking.Entry) error {
	if m.closed {
		return ErrStorageClosed
	}
	if _, exists := m.entries[entry.ID]; !exists {
		return &NotFoundError{ID: entry.ID}
	}
	m.entries[entry.ID] = entry
	return nil
}

func (m *MockStorage) Delete(ctx context.Context, id string) error {
	if m.closed {
		return ErrStorageClosed
	}
	if _, exists := m.entries[id]; !exists {
		return &NotFoundError{ID: id}
	}
	delete(m.entries, id)
	return nil
}

func (m *MockStorage) List(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	result := make([]tracking.Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		result = append(result, *entry)
	}
	return result, nil
}

func (m *MockStorage) FindByQuestion(ctx context.Context, question string) (*tracking.Entry, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	for _, entry := range m.entries {
		if entry.CanonicalQuestion == question {
			return entry, nil
		}
	}
	return nil, &NotFoundError{ID: question}
}

func (m *MockStorage) GetByTags(ctx context.Context, tags []string) ([]tracking.Entry, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	return []tracking.Entry{}, nil
}

func (m *MockStorage) GetBySprint(ctx context.Context, sprint string) ([]tracking.Entry, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	return []tracking.Entry{}, nil
}

func (m *MockStorage) BulkInsert(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	result := &BulkResult{}
	for i := range entries {
		if err := m.Create(ctx, &entries[i]); err != nil {
			result.Skipped++
		} else {
			result.Created++
		}
		result.Processed++
	}
	return result, nil
}

func (m *MockStorage) BulkUpdate(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	result := &BulkResult{}
	for i := range entries {
		if err := m.Update(ctx, &entries[i]); err != nil {
			result.Skipped++
		} else {
			result.Updated++
		}
		result.Processed++
	}
	return result, nil
}

func (m *MockStorage) BulkDelete(ctx context.Context, ids []string) (*BulkResult, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	result := &BulkResult{}
	for _, id := range ids {
		if err := m.Delete(ctx, id); err != nil {
			result.Skipped++
		} else {
			result.Processed++
		}
	}
	return result, nil
}

func (m *MockStorage) Import(ctx context.Context, entries []tracking.Entry, mode ImportMode) (*BulkResult, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	if mode == ImportModeOverwrite {
		m.entries = make(map[string]*tracking.Entry)
	}
	return m.BulkInsert(ctx, entries)
}

func (m *MockStorage) Export(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	return m.List(ctx, filter)
}

func (m *MockStorage) Vacuum(ctx context.Context) (int64, error) {
	if m.closed {
		return 0, ErrStorageClosed
	}
	return 0, nil
}

func (m *MockStorage) Stats(ctx context.Context) (*StorageStats, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}
	return &StorageStats{
		TotalEntries: len(m.entries),
	}, nil
}

func (m *MockStorage) Backup(ctx context.Context, path string) error {
	if m.closed {
		return ErrStorageClosed
	}
	return nil
}

func (m *MockStorage) Close() error {
	m.closed = true
	return nil
}

// Verify MockStorage implements Storage interface
var _ Storage = (*MockStorage)(nil)

// TestStorageInterfaceCRUD tests CRUD operations on the Storage interface.
func TestStorageInterfaceCRUD(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	defer store.Close()

	// Test Create
	entry := tracking.NewEntry("test-1", "What is the answer?", "42", "2025-01-01")
	if err := store.Create(ctx, entry); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test duplicate Create
	if err := store.Create(ctx, entry); err == nil {
		t.Fatal("Expected ErrDuplicateEntry, got nil")
	} else if !errors.Is(err, ErrDuplicateEntry) {
		t.Fatalf("Expected ErrDuplicateEntry, got %v", err)
	}

	// Test Read
	retrieved, err := store.Read(ctx, "test-1")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if retrieved.CanonicalQuestion != entry.CanonicalQuestion {
		t.Errorf("Expected question %q, got %q", entry.CanonicalQuestion, retrieved.CanonicalQuestion)
	}

	// Test Read not found
	_, err = store.Read(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected ErrNotFound, got nil")
	} else if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}

	// Test Update
	entry.CurrentAnswer = "Updated answer"
	if err := store.Update(ctx, entry); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, _ = store.Read(ctx, "test-1")
	if retrieved.CurrentAnswer != "Updated answer" {
		t.Errorf("Expected updated answer, got %q", retrieved.CurrentAnswer)
	}

	// Test Delete
	if err := store.Delete(ctx, "test-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify delete
	_, err = store.Read(ctx, "test-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Expected ErrNotFound after delete, got %v", err)
	}
}

// TestStorageInterfaceQuery tests query operations.
func TestStorageInterfaceQuery(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	defer store.Close()

	// Add test entries
	entries := []tracking.Entry{
		*tracking.NewEntry("q1", "First question?", "Answer 1", "2025-01-01"),
		*tracking.NewEntry("q2", "Second question?", "Answer 2", "2025-01-02"),
	}
	for i := range entries {
		store.Create(ctx, &entries[i])
	}

	// Test List
	list, err := store.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(list))
	}

	// Test FindByQuestion
	found, err := store.FindByQuestion(ctx, "First question?")
	if err != nil {
		t.Fatalf("FindByQuestion failed: %v", err)
	}
	if found.ID != "q1" {
		t.Errorf("Expected ID q1, got %s", found.ID)
	}
}

// TestStorageInterfaceBulk tests bulk operations.
func TestStorageInterfaceBulk(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	defer store.Close()

	entries := []tracking.Entry{
		*tracking.NewEntry("b1", "Bulk 1?", "A1", "2025-01-01"),
		*tracking.NewEntry("b2", "Bulk 2?", "A2", "2025-01-02"),
		*tracking.NewEntry("b3", "Bulk 3?", "A3", "2025-01-03"),
	}

	// Test BulkInsert
	result, err := store.BulkInsert(ctx, entries)
	if err != nil {
		t.Fatalf("BulkInsert failed: %v", err)
	}
	if result.Created != 3 {
		t.Errorf("Expected 3 created, got %d", result.Created)
	}

	// Test BulkDelete
	result, err = store.BulkDelete(ctx, []string{"b1", "b2"})
	if err != nil {
		t.Fatalf("BulkDelete failed: %v", err)
	}
	if result.Processed != 2 {
		t.Errorf("Expected 2 processed, got %d", result.Processed)
	}

	// Verify remaining
	list, _ := store.List(ctx, ListFilter{})
	if len(list) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(list))
	}
}

// TestStorageInterfaceMaintenance tests maintenance operations.
func TestStorageInterfaceMaintenance(t *testing.T) {
	ctx := context.Background()
	store := NewMockStorage()
	defer store.Close()

	// Test Stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries, got %d", stats.TotalEntries)
	}

	// Test Vacuum
	_, err = store.Vacuum(ctx)
	if err != nil {
		t.Fatalf("Vacuum failed: %v", err)
	}

	// Test Close and subsequent operations
	store.Close()
	_, err = store.Read(ctx, "test")
	if !errors.Is(err, ErrStorageClosed) {
		t.Fatalf("Expected ErrStorageClosed, got %v", err)
	}
}

// TestErrorTypes tests error type unwrapping.
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		sentinel error
	}{
		{"NotFoundError", &NotFoundError{ID: "test"}, ErrNotFound},
		{"DuplicateEntryError", &DuplicateEntryError{ID: "test"}, ErrDuplicateEntry},
		{"ConstraintViolationError", &ConstraintViolationError{Message: "test"}, ErrConstraintViolation},
		{"UnsupportedBackendError", &UnsupportedBackendError{Extension: ".txt"}, ErrUnsupportedBackend},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.sentinel) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.sentinel)
			}
		})
	}
}

// TestImportMode tests ImportMode string conversion.
func TestImportMode(t *testing.T) {
	tests := []struct {
		mode ImportMode
		str  string
	}{
		{ImportModeAppend, "append"},
		{ImportModeOverwrite, "overwrite"},
		{ImportModeMerge, "merge"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.str {
				t.Errorf("String() = %q, want %q", got, tt.str)
			}
			if got := ParseImportMode(tt.str); got != tt.mode {
				t.Errorf("ParseImportMode(%q) = %v, want %v", tt.str, got, tt.mode)
			}
		})
	}
}
