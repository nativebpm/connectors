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

// NewSqliteSnapshotStore initializes a new SQLite snapshot store and creates all required tables.
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

	// Create table to hold full memory snapshots
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

	// Create table to hold memory deltas (Dirty-Pages)
	createDeltasTableSQL := `
	CREATE TABLE IF NOT EXISTS memory_deltas (
		instance_id TEXT,
		page_index INTEGER,
		data BLOB NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (instance_id, page_index)
	);`
	_, err = db.Exec(createDeltasTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create memory_deltas table: %w", err)
	}

	// Create table to hold operation logs (Oplog)
	createOplogTableSQL := `
	CREATE TABLE IF NOT EXISTS oplog (
		instance_id TEXT,
		call_index INTEGER,
		api_name TEXT NOT NULL,
		request_payload BLOB,
		response_payload BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (instance_id, call_index)
	);`
	_, err = db.Exec(createOplogTableSQL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create oplog table: %w", err)
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

// SaveDeltas saves dirty pages in transaction
func (s *SqliteSnapshotStore) SaveDeltas(id string, deltas map[int][]byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for SaveDeltas: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
	INSERT INTO memory_deltas (instance_id, page_index, data, updated_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(instance_id, page_index) DO UPDATE SET
		data = excluded.data,
		updated_at = CURRENT_TIMESTAMP;`

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare SaveDeltas query: %w", err)
	}
	defer stmt.Close()

	for pageIdx, data := range deltas {
		_, err := stmt.Exec(id, pageIdx, data)
		if err != nil {
			return fmt.Errorf("failed to save memory delta page %d: %w", pageIdx, err)
		}
	}

	return tx.Commit()
}

// LoadDeltas retrieves delta pages from the database
func (s *SqliteSnapshotStore) LoadDeltas(id string) (map[int][]byte, error) {
	query := `SELECT page_index, data FROM memory_deltas WHERE instance_id = ?;`
	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory deltas: %w", err)
	}
	defer rows.Close()

	deltas := make(map[int][]byte)
	for rows.Next() {
		var pageIdx int
		var data []byte
		if err := rows.Scan(&pageIdx, &data); err != nil {
			return nil, fmt.Errorf("failed to scan memory delta row: %w", err)
		}
		deltas[pageIdx] = data
	}
	return deltas, nil
}

// SaveOplog records API call in database
func (s *SqliteSnapshotStore) SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error {
	query := `
	INSERT INTO oplog (instance_id, call_index, api_name, request_payload, response_payload, created_at)
	VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(instance_id, call_index) DO UPDATE SET
		api_name = excluded.api_name,
		request_payload = excluded.request_payload,
		response_payload = excluded.response_payload;`

	_, err := s.db.Exec(query, id, callIndex, apiName, request, response)
	if err != nil {
		return fmt.Errorf("failed to save oplog: %w", err)
	}
	return nil
}

// LoadOplog retrieves the execution log
func (s *SqliteSnapshotStore) LoadOplog(id string) ([]OplogEntry, error) {
	query := `SELECT call_index, api_name, request_payload, response_payload FROM oplog WHERE instance_id = ? ORDER BY call_index ASC;`
	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query oplog: %w", err)
	}
	defer rows.Close()

	var list []OplogEntry
	for rows.Next() {
		var entry OplogEntry
		if err := rows.Scan(&entry.CallIndex, &entry.ApiName, &entry.RequestPayload, &entry.ResponsePayload); err != nil {
			return nil, fmt.Errorf("failed to scan oplog row: %w", err)
		}
		list = append(list, entry)
	}
	return list, nil
}

// Delete removes all data associated with the instance from all tables.
func (s *SqliteSnapshotStore) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("DELETE FROM snapshots WHERE id = ?;", id)
	_, _ = tx.Exec("DELETE FROM memory_deltas WHERE instance_id = ?;", id)
	_, _ = tx.Exec("DELETE FROM oplog WHERE instance_id = ?;", id)

	return tx.Commit()
}

// Close gracefully closes the SQLite database connection.
func (s *SqliteSnapshotStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
