package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage/schema"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	_ "modernc.org/sqlite"
)

// SQLiteStorage implements Storage interface using SQLite database.
type SQLiteStorage struct {
	db     *sql.DB
	path   string
	closed bool
}

// NewSQLiteStorage creates a new SQLite storage instance.
func NewSQLiteStorage(ctx context.Context, path string) (*SQLiteStorage, error) {
	if path == "" {
		return nil, ErrInvalidPath
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		return nil, &UnsupportedBackendError{Extension: ext}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database with connection parameters
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Initialize schema
	if _, err := schema.Initialize(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStorage{
		db:   db,
		path: path,
	}, nil
}

// Create adds a new entry to storage.
func (s *SQLiteStorage) Create(ctx context.Context, entry *tracking.Entry) error {
	if s.closed {
		return ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check for existing entry
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM entries WHERE id = ?", entry.ID).Scan(&exists)
	if err == nil {
		return &DuplicateEntryError{ID: entry.ID}
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing entry: %w", err)
	}

	// Insert entry
	now := time.Now().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, occurrences,
			first_seen, last_seen, status, confidence, promoted_to, promoted_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ID, entry.CanonicalQuestion, entry.CurrentAnswer, entry.Occurrences,
		entry.FirstSeen, entry.LastSeen, entry.Status, entry.Confidence,
		nullableString(entry.PromotedTo), nullableString(entry.PromotedDate), now, now)
	if err != nil {
		return fmt.Errorf("failed to insert entry: %w", err)
	}

	// Insert variants
	if err := s.insertVariants(ctx, tx, entry.ID, entry.Variants); err != nil {
		return err
	}

	// Insert tags
	if err := s.insertTags(ctx, tx, entry.ID, entry.ContextTags); err != nil {
		return err
	}

	// Insert sprints
	if err := s.insertSprints(ctx, tx, entry.ID, entry.SprintsSeen); err != nil {
		return err
	}

	return tx.Commit()
}

// Read retrieves an entry by ID.
func (s *SQLiteStorage) Read(ctx context.Context, id string) (*tracking.Entry, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	entry := &tracking.Entry{}
	var promotedTo, promotedDate sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, canonical_question, current_answer, occurrences,
			first_seen, last_seen, status, confidence, promoted_to, promoted_date
		FROM entries WHERE id = ?
	`, id).Scan(&entry.ID, &entry.CanonicalQuestion, &entry.CurrentAnswer, &entry.Occurrences,
		&entry.FirstSeen, &entry.LastSeen, &entry.Status, &entry.Confidence,
		&promotedTo, &promotedDate)
	if err == sql.ErrNoRows {
		return nil, &NotFoundError{ID: id}
	} else if err != nil {
		return nil, fmt.Errorf("failed to read entry: %w", err)
	}

	entry.PromotedTo = promotedTo.String
	entry.PromotedDate = promotedDate.String

	// Load related data
	if err := s.loadRelatedData(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// Update modifies an existing entry.
func (s *SQLiteStorage) Update(ctx context.Context, entry *tracking.Entry) error {
	if s.closed {
		return ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check entry exists
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM entries WHERE id = ?", entry.ID).Scan(&exists)
	if err == sql.ErrNoRows {
		return &NotFoundError{ID: entry.ID}
	} else if err != nil {
		return fmt.Errorf("failed to check existing entry: %w", err)
	}

	// Update entry
	now := time.Now().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, `
		UPDATE entries SET canonical_question = ?, current_answer = ?, occurrences = ?,
			first_seen = ?, last_seen = ?, status = ?, confidence = ?,
			promoted_to = ?, promoted_date = ?, updated_at = ?
		WHERE id = ?
	`, entry.CanonicalQuestion, entry.CurrentAnswer, entry.Occurrences,
		entry.FirstSeen, entry.LastSeen, entry.Status, entry.Confidence,
		nullableString(entry.PromotedTo), nullableString(entry.PromotedDate), now, entry.ID)
	if err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Update variants (delete and reinsert)
	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_variants WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("failed to delete variants: %w", err)
	}
	if err := s.insertVariants(ctx, tx, entry.ID, entry.Variants); err != nil {
		return err
	}

	// Update tags
	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}
	if err := s.insertTags(ctx, tx, entry.ID, entry.ContextTags); err != nil {
		return err
	}

	// Update sprints
	if _, err := tx.ExecContext(ctx, "DELETE FROM entry_sprints WHERE entry_id = ?", entry.ID); err != nil {
		return fmt.Errorf("failed to delete sprints: %w", err)
	}
	if err := s.insertSprints(ctx, tx, entry.ID, entry.SprintsSeen); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete removes an entry by ID.
func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	if s.closed {
		return ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}
	if affected == 0 {
		return &NotFoundError{ID: id}
	}

	return nil
}

// List returns entries matching the filter criteria.
func (s *SQLiteStorage) List(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	query, args := s.buildListQuery(filter)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	entries := []tracking.Entry{}
	for rows.Next() {
		var entry tracking.Entry
		var promotedTo, promotedDate sql.NullString

		err := rows.Scan(&entry.ID, &entry.CanonicalQuestion, &entry.CurrentAnswer, &entry.Occurrences,
			&entry.FirstSeen, &entry.LastSeen, &entry.Status, &entry.Confidence,
			&promotedTo, &promotedDate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}

		entry.PromotedTo = promotedTo.String
		entry.PromotedDate = promotedDate.String

		if err := s.loadRelatedData(ctx, &entry); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// FindByQuestion searches for an entry by canonical question.
func (s *SQLiteStorage) FindByQuestion(ctx context.Context, question string) (*tracking.Entry, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	entry := &tracking.Entry{}
	var promotedTo, promotedDate sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, canonical_question, current_answer, occurrences,
			first_seen, last_seen, status, confidence, promoted_to, promoted_date
		FROM entries WHERE canonical_question = ?
	`, question).Scan(&entry.ID, &entry.CanonicalQuestion, &entry.CurrentAnswer, &entry.Occurrences,
		&entry.FirstSeen, &entry.LastSeen, &entry.Status, &entry.Confidence,
		&promotedTo, &promotedDate)
	if err == sql.ErrNoRows {
		return nil, &NotFoundError{ID: question}
	} else if err != nil {
		return nil, fmt.Errorf("failed to find entry: %w", err)
	}

	entry.PromotedTo = promotedTo.String
	entry.PromotedDate = promotedDate.String

	if err := s.loadRelatedData(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// GetByTags returns entries containing any of the specified tags.
func (s *SQLiteStorage) GetByTags(ctx context.Context, tags []string) ([]tracking.Entry, error) {
	return s.List(ctx, ListFilter{Tags: tags})
}

// GetBySprint returns entries seen in the specified sprint.
func (s *SQLiteStorage) GetBySprint(ctx context.Context, sprint string) ([]tracking.Entry, error) {
	return s.List(ctx, ListFilter{Sprint: sprint})
}

// BulkInsert adds multiple entries in a single transaction.
func (s *SQLiteStorage) BulkInsert(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result := &BulkResult{}
	now := time.Now().Format(time.RFC3339)

	for i := range entries {
		result.Processed++

		// Check for existing
		var exists int
		err := tx.QueryRowContext(ctx, "SELECT 1 FROM entries WHERE id = ?", entries[i].ID).Scan(&exists)
		if err == nil {
			result.Skipped++
			continue
		} else if err != sql.ErrNoRows {
			result.Errors = append(result.Errors, err)
			continue
		}

		// Insert entry
		_, err = tx.ExecContext(ctx, `
			INSERT INTO entries (id, canonical_question, current_answer, occurrences,
				first_seen, last_seen, status, confidence, promoted_to, promoted_date, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, entries[i].ID, entries[i].CanonicalQuestion, entries[i].CurrentAnswer, entries[i].Occurrences,
			entries[i].FirstSeen, entries[i].LastSeen, entries[i].Status, entries[i].Confidence,
			nullableString(entries[i].PromotedTo), nullableString(entries[i].PromotedDate), now, now)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		if err := s.insertVariants(ctx, tx, entries[i].ID, entries[i].Variants); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if err := s.insertTags(ctx, tx, entries[i].ID, entries[i].ContextTags); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if err := s.insertSprints(ctx, tx, entries[i].ID, entries[i].SprintsSeen); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		result.Created++
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit: %w", err)
	}

	return result, nil
}

// BulkUpdate modifies multiple entries in a single transaction.
func (s *SQLiteStorage) BulkUpdate(ctx context.Context, entries []tracking.Entry) (*BulkResult, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result := &BulkResult{}
	now := time.Now().Format(time.RFC3339)

	for i := range entries {
		result.Processed++

		res, err := tx.ExecContext(ctx, `
			UPDATE entries SET canonical_question = ?, current_answer = ?, occurrences = ?,
				first_seen = ?, last_seen = ?, status = ?, confidence = ?,
				promoted_to = ?, promoted_date = ?, updated_at = ?
			WHERE id = ?
		`, entries[i].CanonicalQuestion, entries[i].CurrentAnswer, entries[i].Occurrences,
			entries[i].FirstSeen, entries[i].LastSeen, entries[i].Status, entries[i].Confidence,
			nullableString(entries[i].PromotedTo), nullableString(entries[i].PromotedDate), now, entries[i].ID)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}

		affected, _ := res.RowsAffected()
		if affected == 0 {
			result.Skipped++
			continue
		}

		// Update related data
		tx.ExecContext(ctx, "DELETE FROM entry_variants WHERE entry_id = ?", entries[i].ID)
		tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entries[i].ID)
		tx.ExecContext(ctx, "DELETE FROM entry_sprints WHERE entry_id = ?", entries[i].ID)
		s.insertVariants(ctx, tx, entries[i].ID, entries[i].Variants)
		s.insertTags(ctx, tx, entries[i].ID, entries[i].ContextTags)
		s.insertSprints(ctx, tx, entries[i].ID, entries[i].SprintsSeen)

		result.Updated++
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("failed to commit: %w", err)
	}

	return result, nil
}

// BulkDelete removes multiple entries by ID.
func (s *SQLiteStorage) BulkDelete(ctx context.Context, ids []string) (*BulkResult, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	result := &BulkResult{}
	for _, id := range ids {
		res, err := s.db.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", id)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			result.Processed++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

// Import loads entries with specified mode.
func (s *SQLiteStorage) Import(ctx context.Context, entries []tracking.Entry, mode ImportMode) (*BulkResult, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	switch mode {
	case ImportModeOverwrite:
		// Delete all existing entries
		if _, err := s.db.ExecContext(ctx, "DELETE FROM entries"); err != nil {
			return nil, fmt.Errorf("failed to clear entries: %w", err)
		}
		return s.BulkInsert(ctx, entries)

	case ImportModeMerge:
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		result := &BulkResult{}
		now := time.Now().Format(time.RFC3339)

		for i := range entries {
			result.Processed++

			var exists int
			err := tx.QueryRowContext(ctx, "SELECT 1 FROM entries WHERE id = ?", entries[i].ID).Scan(&exists)
			if err == sql.ErrNoRows {
				// Insert new
				_, err = tx.ExecContext(ctx, `
					INSERT INTO entries (id, canonical_question, current_answer, occurrences,
						first_seen, last_seen, status, confidence, promoted_to, promoted_date, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`, entries[i].ID, entries[i].CanonicalQuestion, entries[i].CurrentAnswer, entries[i].Occurrences,
					entries[i].FirstSeen, entries[i].LastSeen, entries[i].Status, entries[i].Confidence,
					nullableString(entries[i].PromotedTo), nullableString(entries[i].PromotedDate), now, now)
				if err == nil {
					s.insertVariants(ctx, tx, entries[i].ID, entries[i].Variants)
					s.insertTags(ctx, tx, entries[i].ID, entries[i].ContextTags)
					s.insertSprints(ctx, tx, entries[i].ID, entries[i].SprintsSeen)
					result.Created++
				}
			} else if err == nil {
				// Update existing
				_, err = tx.ExecContext(ctx, `
					UPDATE entries SET canonical_question = ?, current_answer = ?, occurrences = ?,
						first_seen = ?, last_seen = ?, status = ?, confidence = ?,
						promoted_to = ?, promoted_date = ?, updated_at = ?
					WHERE id = ?
				`, entries[i].CanonicalQuestion, entries[i].CurrentAnswer, entries[i].Occurrences,
					entries[i].FirstSeen, entries[i].LastSeen, entries[i].Status, entries[i].Confidence,
					nullableString(entries[i].PromotedTo), nullableString(entries[i].PromotedDate), now, entries[i].ID)
				if err == nil {
					tx.ExecContext(ctx, "DELETE FROM entry_variants WHERE entry_id = ?", entries[i].ID)
					tx.ExecContext(ctx, "DELETE FROM entry_tags WHERE entry_id = ?", entries[i].ID)
					tx.ExecContext(ctx, "DELETE FROM entry_sprints WHERE entry_id = ?", entries[i].ID)
					s.insertVariants(ctx, tx, entries[i].ID, entries[i].Variants)
					s.insertTags(ctx, tx, entries[i].ID, entries[i].ContextTags)
					s.insertSprints(ctx, tx, entries[i].ID, entries[i].SprintsSeen)
					result.Updated++
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("failed to commit: %w", err)
		}
		return result, nil

	default: // ImportModeAppend
		return s.BulkInsert(ctx, entries)
	}
}

// Export returns all entries matching the filter.
func (s *SQLiteStorage) Export(ctx context.Context, filter ListFilter) ([]tracking.Entry, error) {
	return s.List(ctx, filter)
}

// Vacuum performs storage optimization.
func (s *SQLiteStorage) Vacuum(ctx context.Context) (int64, error) {
	if s.closed {
		return 0, ErrStorageClosed
	}

	// Get size before vacuum
	info, err := os.Stat(s.path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	sizeBefore := info.Size()

	// Run VACUUM
	if _, err := s.db.ExecContext(ctx, "VACUUM"); err != nil {
		return 0, fmt.Errorf("vacuum failed: %w", err)
	}

	// Get size after
	info, err = os.Stat(s.path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file after vacuum: %w", err)
	}
	sizeAfter := info.Size()

	return sizeBefore - sizeAfter, nil
}

// Stats returns storage statistics.
func (s *SQLiteStorage) Stats(ctx context.Context) (*StorageStats, error) {
	if s.closed {
		return nil, ErrStorageClosed
	}

	stats := &StorageStats{
		EntriesByStatus: make(map[string]int),
	}

	// Total entries
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries").Scan(&stats.TotalEntries)

	// Entries by status
	rows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM entries GROUP BY status")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if rows.Scan(&status, &count) == nil {
				stats.EntriesByStatus[status] = count
			}
		}
	}

	// Total variants
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entry_variants").Scan(&stats.TotalVariants)

	// Total tags
	s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT tag) FROM entry_tags").Scan(&stats.TotalTags)

	// Total sprints
	s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT sprint) FROM entry_sprints").Scan(&stats.TotalSprints)

	// Last modified
	s.db.QueryRowContext(ctx, "SELECT MAX(updated_at) FROM entries").Scan(&stats.LastModified)

	// Storage size
	if info, err := os.Stat(s.path); err == nil {
		stats.StorageSize = info.Size()
	}

	return stats, nil
}

// Backup creates a backup of the database.
func (s *SQLiteStorage) Backup(ctx context.Context, path string) error {
	if s.closed {
		return ErrStorageClosed
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Use SQLite backup API via VACUUM INTO
	_, err := s.db.ExecContext(ctx, "VACUUM INTO ?", path)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	return nil
}

// Close releases resources.
func (s *SQLiteStorage) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}

// Helper methods

func (s *SQLiteStorage) insertVariants(ctx context.Context, tx *sql.Tx, entryID string, variants []string) error {
	for _, v := range variants {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_variants (entry_id, variant) VALUES (?, ?)", entryID, v); err != nil {
			return fmt.Errorf("failed to insert variant: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStorage) insertTags(ctx context.Context, tx *sql.Tx, entryID string, tags []string) error {
	for _, t := range tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)", entryID, t); err != nil {
			return fmt.Errorf("failed to insert tag: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStorage) insertSprints(ctx context.Context, tx *sql.Tx, entryID string, sprints []string) error {
	for _, sp := range sprints {
		if _, err := tx.ExecContext(ctx, "INSERT INTO entry_sprints (entry_id, sprint) VALUES (?, ?)", entryID, sp); err != nil {
			return fmt.Errorf("failed to insert sprint: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStorage) loadRelatedData(ctx context.Context, entry *tracking.Entry) error {
	// Load variants
	rows, err := s.db.QueryContext(ctx, "SELECT variant FROM entry_variants WHERE entry_id = ?", entry.ID)
	if err != nil {
		return fmt.Errorf("failed to load variants: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if rows.Scan(&v) == nil {
			entry.Variants = append(entry.Variants, v)
		}
	}

	// Load tags
	rows, err = s.db.QueryContext(ctx, "SELECT tag FROM entry_tags WHERE entry_id = ?", entry.ID)
	if err != nil {
		return fmt.Errorf("failed to load tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil {
			entry.ContextTags = append(entry.ContextTags, t)
		}
	}

	// Load sprints
	rows, err = s.db.QueryContext(ctx, "SELECT sprint FROM entry_sprints WHERE entry_id = ?", entry.ID)
	if err != nil {
		return fmt.Errorf("failed to load sprints: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sp string
		if rows.Scan(&sp) == nil {
			entry.SprintsSeen = append(entry.SprintsSeen, sp)
		}
	}

	return nil
}

func (s *SQLiteStorage) buildListQuery(filter ListFilter) (string, []interface{}) {
	query := `
		SELECT DISTINCT e.id, e.canonical_question, e.current_answer, e.occurrences,
			e.first_seen, e.last_seen, e.status, e.confidence, e.promoted_to, e.promoted_date
		FROM entries e
	`
	var args []interface{}
	var conditions []string

	// Join with tags if filtering by tags
	if len(filter.Tags) > 0 {
		query += " JOIN entry_tags et ON e.id = et.entry_id"
		placeholders := make([]string, len(filter.Tags))
		for i, tag := range filter.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf("et.tag IN (%s)", strings.Join(placeholders, ",")))
	}

	// Join with sprints if filtering by sprint
	if filter.Sprint != "" {
		query += " JOIN entry_sprints es ON e.id = es.entry_id"
		conditions = append(conditions, "es.sprint = ?")
		args = append(args, filter.Sprint)
	}

	// Filter by status
	if filter.Status != "" {
		conditions = append(conditions, "e.status = ?")
		args = append(args, filter.Status)
	}

	// Filter by min occurrences
	if filter.MinOccurrences > 0 {
		conditions = append(conditions, "e.occurrences >= ?")
		args = append(args, filter.MinOccurrences)
	}

	// Full-text search
	if filter.Query != "" {
		query = `
			SELECT DISTINCT e.id, e.canonical_question, e.current_answer, e.occurrences,
				e.first_seen, e.last_seen, e.status, e.confidence, e.promoted_to, e.promoted_date
			FROM entries e
			JOIN entries_fts f ON e.id = f.id
		`
		conditions = append(conditions, "entries_fts MATCH ?")
		args = append(args, filter.Query)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY e.last_seen DESC"

	// Pagination
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	return query, args
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Verify SQLiteStorage implements Storage interface
var _ Storage = (*SQLiteStorage)(nil)
