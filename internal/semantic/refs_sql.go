package semantic

import (
	"context"
	"database/sql"
	"strings"
)

// RefEdge is a chunk reference paired with the location of the chunk at the far
// end of the edge: the caller for GetCallersByName, the callee for
// GetRefsByName. Location fields stay empty when that end is unresolved.
type RefEdge struct {
	ChunkRef
	FilePath  string `json:"file_path,omitempty"`
	Name      string `json:"name,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// Resolved reports whether this edge points at a known chunk. An unresolved
// edge names a symbol outside the index, such as a standard library call.
func (e RefEdge) Resolved() bool { return e.RefTargetID != "" }

// baseRefName returns the final segment of a possibly-qualified reference:
// "fmt.Println" and "recv.Method" become "Println" and "Method". Chunk names
// are always bare identifiers, so this is the only form that can match one.
func baseRefName(refName string) string {
	if i := strings.LastIndex(refName, "."); i >= 0 {
		return refName[i+1:]
	}
	return refName
}

// chunkRefsTableDDL creates the reference table. Index creation is deliberately
// separate: a database written before ref_base_name existed needs the column
// migration to run before any index can name that column.
const chunkRefsTableDDL = `
CREATE TABLE IF NOT EXISTS chunk_refs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	chunk_id TEXT NOT NULL,
	ref_type TEXT NOT NULL,
	ref_name TEXT NOT NULL,
	ref_base_name TEXT,
	ref_target_id TEXT,
	FOREIGN KEY (chunk_id) REFERENCES chunks(id) ON DELETE CASCADE
);`

const chunkRefsIndexDDL = `
CREATE INDEX IF NOT EXISTS idx_chunk_refs_chunk_id ON chunk_refs(chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunk_refs_ref_name ON chunk_refs(ref_name);
CREATE INDEX IF NOT EXISTS idx_chunk_refs_target ON chunk_refs(ref_target_id);
CREATE INDEX IF NOT EXISTS idx_chunk_refs_base_name ON chunk_refs(ref_base_name);
CREATE INDEX IF NOT EXISTS idx_chunks_name ON chunks(name);
`

// initChunkRefsTables creates or upgrades the reference tables. It is safe to
// call on a database created by an older build: the table is left in place and
// only the missing column and indexes are added.
func initChunkRefsTables(db *sql.DB) error {
	if _, err := db.Exec(chunkRefsTableDDL); err != nil {
		return err
	}
	if err := migrateRefBaseNameColumn(db); err != nil {
		return err
	}
	_, err := db.Exec(chunkRefsIndexDDL)
	return err
}

// migrateRefBaseNameColumn adds ref_base_name to databases written before the
// column existed, and backfills it so previously stored edges stay resolvable.
func migrateRefBaseNameColumn(db *sql.DB) error {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('chunk_refs')
		WHERE name = 'ref_base_name'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE chunk_refs ADD COLUMN ref_base_name TEXT`); err != nil {
		return err
	}

	// Backfill legacy rows. Splitting on the first dot is exact for the bare
	// and single-qualifier forms that make up practically all references; a
	// deeper name simply stays unresolved until its chunk is indexed again.
	_, err = db.Exec(`
		UPDATE chunk_refs
		SET ref_base_name = CASE
			WHEN instr(ref_name, '.') > 0 THEN substr(ref_name, instr(ref_name, '.') + 1)
			ELSE ref_name
		END
		WHERE ref_base_name IS NULL
	`)
	return err
}

// storeRefsSQL inserts references in a single transaction, deriving the base
// name each edge will later be resolved on.
func storeRefsSQL(ctx context.Context, db *sql.DB, refs []ChunkRef) error {
	if len(refs) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunk_refs (chunk_id, ref_type, ref_name, ref_base_name, ref_target_id)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ref := range refs {
		_, err = stmt.ExecContext(ctx, ref.ChunkID, string(ref.RefType), ref.RefName,
			baseRefName(ref.RefName), nullableString(ref.RefTargetID))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

const refColumns = `chunk_id, ref_type, ref_name, COALESCE(ref_target_id, '')`

func queryRefsSQL(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]ChunkRef, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ChunkRef
	for rows.Next() {
		var r ChunkRef
		if err := rows.Scan(&r.ChunkID, &r.RefType, &r.RefName, &r.RefTargetID); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func queryRefEdgesSQL(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]RefEdge, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []RefEdge
	for rows.Next() {
		var e RefEdge
		if err := rows.Scan(&e.ChunkID, &e.RefType, &e.RefName, &e.RefTargetID,
			&e.FilePath, &e.Name, &e.StartLine, &e.EndLine); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// getRefsSQL returns all references leaving a chunk.
func getRefsSQL(ctx context.Context, db *sql.DB, chunkID string) ([]ChunkRef, error) {
	return queryRefsSQL(ctx, db,
		`SELECT `+refColumns+` FROM chunk_refs WHERE chunk_id = ?`, chunkID)
}

// getCallersSQL returns all references arriving at a chunk.
func getCallersSQL(ctx context.Context, db *sql.DB, chunkID string) ([]ChunkRef, error) {
	return queryRefsSQL(ctx, db,
		`SELECT `+refColumns+` FROM chunk_refs WHERE ref_target_id = ?`, chunkID)
}

// deleteRefsByChunkSQL removes every reference leaving a chunk.
func deleteRefsByChunkSQL(ctx context.Context, db *sql.DB, chunkID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM chunk_refs WHERE chunk_id = ?`, chunkID)
	return err
}

// getCallersByNameSQL returns the call edges arriving at any chunk with this
// name, each carrying the location of the chunk the edge starts from.
//
// Only call edges count as callers. A single method call also records a
// uses_type edge under the same name, so without this filter every caller of a
// method would be reported twice.
func getCallersByNameSQL(ctx context.Context, db *sql.DB, name string) ([]RefEdge, error) {
	return queryRefEdgesSQL(ctx, db, `
		SELECT r.chunk_id, r.ref_type, r.ref_name, COALESCE(r.ref_target_id, ''),
		       COALESCE(c.file_path, ''), COALESCE(c.name, ''),
		       COALESCE(c.start_line, 0), COALESCE(c.end_line, 0)
		FROM chunk_refs r
		JOIN chunks c ON c.id = r.chunk_id
		WHERE r.ref_type = '`+string(RefCalls)+`'
		  AND r.ref_target_id IN (SELECT id FROM chunks WHERE name = ?)
		ORDER BY c.file_path, c.start_line
	`, name)
}

// getRefsByNameSQL returns the edges leaving any chunk with this name, each
// carrying the location of the chunk it points at. Unresolved edges are kept
// and carry no location.
func getRefsByNameSQL(ctx context.Context, db *sql.DB, name string) ([]RefEdge, error) {
	return queryRefEdgesSQL(ctx, db, `
		SELECT r.chunk_id, r.ref_type, r.ref_name, COALESCE(r.ref_target_id, ''),
		       COALESCE(t.file_path, ''), COALESCE(t.name, ''),
		       COALESCE(t.start_line, 0), COALESCE(t.end_line, 0)
		FROM chunk_refs r
		LEFT JOIN chunks t ON t.id = r.ref_target_id
		WHERE r.chunk_id IN (SELECT id FROM chunks WHERE name = ?)
		ORDER BY r.ref_type, r.ref_name
	`, name)
}

// resolveRefsSQL links stored references to the chunks they name.
//
// Two rules make the result trustworthy. A reference whose target chunk has
// disappeared is unlinked first, so a rename cannot leave an edge pointing at a
// chunk that no longer exists. A reference whose name matches more than one
// chunk is left unresolved, because a confidently wrong edge is worse than a
// missing one.
func resolveRefsSQL(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		UPDATE chunk_refs
		SET ref_target_id = NULL
		WHERE ref_target_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM chunks c WHERE c.id = chunk_refs.ref_target_id)
	`)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		UPDATE chunk_refs
		SET ref_target_id = (
			SELECT CASE WHEN COUNT(*) = 1 THEN MIN(c.id) END
			FROM chunks c
			WHERE c.name = chunk_refs.ref_base_name
		)
		WHERE ref_target_id IS NULL
	`)
	return err
}
