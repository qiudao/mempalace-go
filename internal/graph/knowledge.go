// Package graph provides a temporal knowledge graph backed by SQLite.
package graph

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Triple represents a subject-predicate-object relationship with temporal bounds.
type Triple struct {
	Subject   string
	Predicate string
	Object    string
	ValidFrom string
	ValidTo   string // empty if still active
}

// KGStats holds aggregate counts for the knowledge graph.
type KGStats struct {
	Entities int
	Triples  int
}

// KnowledgeGraph is a temporal triple store backed by SQLite.
type KnowledgeGraph struct {
	db *sql.DB
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS entities (
	id   INTEGER PRIMARY KEY,
	name TEXT UNIQUE
);
CREATE TABLE IF NOT EXISTS triples (
	id         INTEGER PRIMARY KEY,
	subject_id INTEGER,
	predicate  TEXT,
	object     TEXT,
	valid_from TEXT,
	valid_to   TEXT,
	FOREIGN KEY(subject_id) REFERENCES entities(id)
);
`

// OpenKnowledgeGraph opens (or creates) a knowledge graph database at path.
func OpenKnowledgeGraph(path string) (*KnowledgeGraph, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open kg db: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("init kg schema: %w", err)
	}
	return &KnowledgeGraph{db: db}, nil
}

// Close closes the underlying database connection.
func (kg *KnowledgeGraph) Close() error {
	return kg.db.Close()
}

// ensureEntity returns the entity id for name, inserting if necessary.
func (kg *KnowledgeGraph) ensureEntity(name string) (int64, error) {
	_, err := kg.db.Exec("INSERT OR IGNORE INTO entities (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	var id int64
	err = kg.db.QueryRow("SELECT id FROM entities WHERE name = ?", name).Scan(&id)
	return id, err
}

// AddTriple adds a new temporal triple to the graph.
func (kg *KnowledgeGraph) AddTriple(subject, predicate, object, validFrom string) error {
	sid, err := kg.ensureEntity(subject)
	if err != nil {
		return fmt.Errorf("ensure entity: %w", err)
	}
	_, err = kg.db.Exec(
		"INSERT INTO triples (subject_id, predicate, object, valid_from, valid_to) VALUES (?, ?, ?, ?, '')",
		sid, predicate, object, validFrom,
	)
	return err
}

// QueryEntity returns all currently active triples for the given subject.
func (kg *KnowledgeGraph) QueryEntity(subject string) ([]Triple, error) {
	rows, err := kg.db.Query(`
		SELECT e.name, t.predicate, t.object, t.valid_from, t.valid_to
		FROM triples t
		JOIN entities e ON e.id = t.subject_id
		WHERE e.name = ? AND t.valid_to = ''`,
		subject,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriples(rows)
}

// QueryEntityAt returns triples that were active at the given date (YYYY-MM-DD).
func (kg *KnowledgeGraph) QueryEntityAt(subject, date string) ([]Triple, error) {
	rows, err := kg.db.Query(`
		SELECT e.name, t.predicate, t.object, t.valid_from, t.valid_to
		FROM triples t
		JOIN entities e ON e.id = t.subject_id
		WHERE e.name = ?
		  AND t.valid_from <= ?
		  AND (t.valid_to = '' OR t.valid_to > ?)`,
		subject, date, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriples(rows)
}

// Invalidate marks a matching active triple as ended at validTo.
func (kg *KnowledgeGraph) Invalidate(subject, predicate, object, validTo string) error {
	_, err := kg.db.Exec(`
		UPDATE triples SET valid_to = ?
		WHERE subject_id = (SELECT id FROM entities WHERE name = ?)
		  AND predicate = ?
		  AND object = ?
		  AND valid_to = ''`,
		validTo, subject, predicate, object,
	)
	return err
}

// Timeline returns all triples (active and invalidated) for subject, ordered by valid_from.
func (kg *KnowledgeGraph) Timeline(subject string) ([]Triple, error) {
	rows, err := kg.db.Query(`
		SELECT e.name, t.predicate, t.object, t.valid_from, t.valid_to
		FROM triples t
		JOIN entities e ON e.id = t.subject_id
		WHERE e.name = ?
		ORDER BY t.valid_from`,
		subject,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriples(rows)
}

// Stats returns aggregate counts of entities and triples.
func (kg *KnowledgeGraph) Stats() (KGStats, error) {
	var s KGStats
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&s.Entities); err != nil {
		return s, err
	}
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM triples").Scan(&s.Triples); err != nil {
		return s, err
	}
	return s, nil
}

func scanTriples(rows *sql.Rows) ([]Triple, error) {
	var out []Triple
	for rows.Next() {
		var t Triple
		if err := rows.Scan(&t.Subject, &t.Predicate, &t.Object, &t.ValidFrom, &t.ValidTo); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
