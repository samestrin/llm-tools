// Package schema handles SQLite database schema creation and migration.
package schema

import (
	"context"
	"database/sql"
	"fmt"
)

// CurrentVersion is the current schema version.
const CurrentVersion = 1

// Schema contains all DDL statements for the clarification database.
const Schema = `
-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Main entries table
CREATE TABLE IF NOT EXISTS entries (
    id TEXT PRIMARY KEY,
    canonical_question TEXT NOT NULL,
    current_answer TEXT,
    occurrences INTEGER DEFAULT 1,
    first_seen TEXT,
    last_seen TEXT,
    status TEXT DEFAULT 'pending',
    confidence TEXT DEFAULT 'low',
    promoted_to TEXT,
    promoted_date TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Normalized array fields
CREATE TABLE IF NOT EXISTS entry_variants (
    entry_id TEXT REFERENCES entries(id) ON DELETE CASCADE,
    variant TEXT,
    PRIMARY KEY (entry_id, variant)
);

CREATE TABLE IF NOT EXISTS entry_tags (
    entry_id TEXT REFERENCES entries(id) ON DELETE CASCADE,
    tag TEXT,
    PRIMARY KEY (entry_id, tag)
);

CREATE TABLE IF NOT EXISTS entry_sprints (
    entry_id TEXT REFERENCES entries(id) ON DELETE CASCADE,
    sprint TEXT,
    PRIMARY KEY (entry_id, sprint)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_entries_status ON entries(status);
CREATE INDEX IF NOT EXISTS idx_entries_occurrences ON entries(occurrences);
CREATE INDEX IF NOT EXISTS idx_entries_last_seen ON entries(last_seen);
CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);
CREATE INDEX IF NOT EXISTS idx_entries_updated_at ON entries(updated_at);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON entry_tags(tag);
CREATE INDEX IF NOT EXISTS idx_sprints_sprint ON entry_sprints(sprint);
`

// FTSSchema contains the full-text search virtual table DDL.
const FTSSchema = `
-- Full-text search on question and answer
CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
    id,
    canonical_question,
    current_answer,
    content='entries',
    content_rowid='rowid'
);

-- Triggers to keep FTS index synchronized with entries table
CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, id, canonical_question, current_answer)
    VALUES (NEW.rowid, NEW.id, NEW.canonical_question, NEW.current_answer);
END;

CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, id, canonical_question, current_answer)
    VALUES('delete', OLD.rowid, OLD.id, OLD.canonical_question, OLD.current_answer);
END;

CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid, id, canonical_question, current_answer)
    VALUES('delete', OLD.rowid, OLD.id, OLD.canonical_question, OLD.current_answer);
    INSERT INTO entries_fts(rowid, id, canonical_question, current_answer)
    VALUES (NEW.rowid, NEW.id, NEW.canonical_question, NEW.current_answer);
END;
`

// Initialize creates the database schema if it doesn't exist.
// Returns the current schema version.
func Initialize(ctx context.Context, db *sql.DB) (int, error) {
	// Enable foreign keys
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return 0, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Check current version
	version, err := GetVersion(ctx, db)
	if err != nil {
		// Table doesn't exist, create schema
		version = 0
	}

	if version >= CurrentVersion {
		return version, nil
	}

	// Create schema in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Apply base schema
	if _, err := tx.ExecContext(ctx, Schema); err != nil {
		return 0, fmt.Errorf("failed to create schema: %w", err)
	}

	// Apply FTS schema
	if _, err := tx.ExecContext(ctx, FTSSchema); err != nil {
		return 0, fmt.Errorf("failed to create FTS schema: %w", err)
	}

	// Record version
	if _, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO schema_version (version) VALUES (?)",
		CurrentVersion); err != nil {
		return 0, fmt.Errorf("failed to record schema version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return CurrentVersion, nil
}

// GetVersion returns the current schema version, or 0 if not initialized.
func GetVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx,
		"SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// Validate checks schema integrity.
func Validate(ctx context.Context, db *sql.DB) error {
	// Check PRAGMA integrity_check
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}

	// Verify required tables exist
	tables := []string{"entries", "entry_variants", "entry_tags", "entry_sprints", "entries_fts"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			table).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if count == 0 {
			return fmt.Errorf("table %s does not exist", table)
		}
	}

	return nil
}
