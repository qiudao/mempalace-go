package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Drawer represents a single memory entry in the palace.
type Drawer struct {
	ID       string
	Document string
	Wing     string
	Room     string
	Source   string
	FiledAt  string
	Hall     string
}

// Query specifies filters for retrieving drawers.
type Query struct {
	Wing   string
	Room   string
	Hall   string
	Limit  int
	Offset int
}

// Store is a SQLite-backed storage engine with FTS5 full-text search.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at path and initialises the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS drawers (
			id       TEXT PRIMARY KEY,
			document TEXT NOT NULL,
			wing     TEXT NOT NULL DEFAULT '',
			room     TEXT NOT NULL DEFAULT '',
			source   TEXT NOT NULL DEFAULT '',
			filed_at TEXT NOT NULL DEFAULT '',
			hall     TEXT NOT NULL DEFAULT ''
		);
		CREATE VIRTUAL TABLE IF NOT EXISTS drawers_fts USING fts5(
			document,
			content='drawers',
			content_rowid='rowid',
			tokenize='porter unicode61'
		);
		CREATE TRIGGER IF NOT EXISTS drawers_ai AFTER INSERT ON drawers BEGIN
			INSERT INTO drawers_fts(rowid, document) VALUES (new.rowid, new.document);
		END;
		CREATE TRIGGER IF NOT EXISTS drawers_ad AFTER DELETE ON drawers BEGIN
			INSERT INTO drawers_fts(drawers_fts, rowid, document) VALUES('delete', old.rowid, old.document);
		END;
		CREATE TRIGGER IF NOT EXISTS drawers_au AFTER UPDATE ON drawers BEGIN
			INSERT INTO drawers_fts(drawers_fts, rowid, document) VALUES('delete', old.rowid, old.document);
			INSERT INTO drawers_fts(rowid, document) VALUES (new.rowid, new.document);
		END;
	`)
	return err
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Add inserts a new drawer. Returns an error if the ID already exists.
func (s *Store) Add(d Drawer) error {
	_, err := s.db.Exec(
		`INSERT INTO drawers (id, document, wing, room, source, filed_at, hall) VALUES (?,?,?,?,?,?,?)`,
		d.ID, d.Document, d.Wing, d.Room, d.Source, d.FiledAt, d.Hall,
	)
	return err
}

// Upsert inserts a drawer or updates it if the ID already exists.
func (s *Store) Upsert(d Drawer) error {
	_, err := s.db.Exec(
		`INSERT INTO drawers (id, document, wing, room, source, filed_at, hall)
		 VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
			document=excluded.document, wing=excluded.wing, room=excluded.room,
			source=excluded.source, filed_at=excluded.filed_at, hall=excluded.hall`,
		d.ID, d.Document, d.Wing, d.Room, d.Source, d.FiledAt, d.Hall,
	)
	return err
}

// Delete removes drawers by their IDs.
func (s *Store) Delete(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := s.db.Exec(
		fmt.Sprintf("DELETE FROM drawers WHERE id IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	return err
}

// Get retrieves drawers matching the query filters.
func (s *Store) Get(q Query) ([]Drawer, error) {
	where, args := q.buildWhere()
	query := "SELECT id, document, wing, room, source, filed_at, hall FROM drawers"
	if where != "" {
		query += " WHERE " + where
	}
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	if q.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.Offset)
	}
	return s.scanDrawers(query, args...)
}

// Count returns the total number of drawers.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM drawers").Scan(&n)
	return n, err
}

func (q Query) buildWhere() (string, []any) {
	var conds []string
	var args []any
	if q.Wing != "" {
		conds = append(conds, "wing = ?")
		args = append(args, q.Wing)
	}
	if q.Room != "" {
		conds = append(conds, "room = ?")
		args = append(args, q.Room)
	}
	if q.Hall != "" {
		conds = append(conds, "hall = ?")
		args = append(args, q.Hall)
	}
	return strings.Join(conds, " AND "), args
}

// SearchResult is a Drawer with its FTS5 BM25 rank score.
type SearchResult struct {
	Drawer
	Rank float64
}

// Search queries the FTS5 index and returns drawers ranked by BM25 relevance.
// Lower rank values indicate better matches. Results can be filtered by Query fields.
// sanitizeFTS5 converts raw text into a safe FTS5 query by extracting
// alphanumeric words and joining them with OR.
func sanitizeFTS5(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_')
	})
	if len(words) == 0 {
		return ""
	}
	// Quote each word to avoid FTS5 syntax issues
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + w + `"`
	}
	return strings.Join(quoted, " OR ")
}

func (s *Store) Search(text string, limit int, q Query) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	ftsQuery := sanitizeFTS5(text)
	if ftsQuery == "" {
		return nil, nil
	}

	where, args := q.buildWhere()

	query := `
		SELECT d.id, d.document, d.wing, d.room, d.source, d.filed_at, d.hall, rank
		FROM drawers_fts f
		JOIN drawers d ON d.rowid = f.rowid
	`
	ftsArgs := []any{ftsQuery}

	if where != "" {
		query += " WHERE drawers_fts MATCH ? AND " + where
		ftsArgs = append(ftsArgs, args...)
	} else {
		query += " WHERE drawers_fts MATCH ?"
	}
	query += " ORDER BY rank LIMIT ?"
	ftsArgs = append(ftsArgs, limit)

	rows, err := s.db.Query(query, ftsArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Document, &r.Wing, &r.Room, &r.Source,
			&r.FiledAt, &r.Hall, &r.Rank); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) scanDrawers(query string, args ...any) ([]Drawer, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Drawer
	for rows.Next() {
		var d Drawer
		if err := rows.Scan(&d.ID, &d.Document, &d.Wing, &d.Room, &d.Source, &d.FiledAt, &d.Hall); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
