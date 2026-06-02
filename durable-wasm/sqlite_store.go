package durable

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SqliteSnapshotStore implements SnapshotStore using a local SQLite database.
type SqliteSnapshotStore struct {
	db *sql.DB
}

// NewSqliteSnapshotStore initializes a new SQLite snapshot store and creates the snapshots table.
func NewSqliteSnapshotStore(dbPath string) (*SqliteSnapshotStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Enable WAL (Write-Ahead Logging) mode which is required by Litestream
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Optimize performance parameters for concurrent reads and writes
	_, err = db.Exec("PRAGMA busy_timeout=5000; PRAGMA synchronous=NORMAL;")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure sqlite pragmas: %w", err)
	}

	// Create table to hold memory snapshots
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS snapshots (
		id TEXT PRIMARY KEY,
		snapshot BLOB NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create snapshots table: %w", err)
	}

	return &SqliteSnapshotStore{db: db}, nil
}

// Save inserts or updates a linear memory snapshot inside the database.
func (s *SqliteSnapshotStore) Save(id string, snapshot []byte) error {
	query := `
	INSERT INTO snapshots (id, snapshot, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		snapshot = excluded.snapshot,
		updated_at = CURRENT_TIMESTAMP;`
	
	_, err := s.db.Exec(query, id, snapshot)
	if err != nil {
		return fmt.Errorf("failed to save snapshot for '%s': %w", id, err)
	}
	return nil
}

// Load retrieves a linear memory snapshot from the database.
func (s *SqliteSnapshotStore) Load(id string) ([]byte, error) {
	query := `SELECT snapshot FROM snapshots WHERE id = ?;`
	
	var snapshot []byte
	err := s.db.QueryRow(query, id).Scan(&snapshot)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	} else if err != nil {
		return nil, fmt.Errorf("failed to load snapshot for '%s': %w", id, err)
	}
	return snapshot, nil
}

// Delete removes a linear memory snapshot from the database.
func (s *SqliteSnapshotStore) Delete(id string) error {
	query := `DELETE FROM snapshots WHERE id = ?;`
	
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot for '%s': %w", id, err)
	}
	return nil
}

// Close gracefully closes the SQLite database connection.
func (s *SqliteSnapshotStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
