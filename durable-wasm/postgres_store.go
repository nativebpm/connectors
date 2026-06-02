package durable

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/lib/pq"
)

//go:embed migrations/postgres/20260602191000_init_postgres.sql
var postgresSchema string

// PostgresSnapshotStore implements SnapshotStore using a PostgreSQL database.
type PostgresSnapshotStore struct {
	db *sql.DB
}

// NewPostgresSnapshotStore initializes a new Postgres snapshot store and creates all required tables.
func NewPostgresSnapshotStore(connStr string) (*PostgresSnapshotStore, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	_, err = db.Exec(postgresSchema)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute postgres schema: %w", err)
	}

	return &PostgresSnapshotStore{db: db}, nil
}

// Save inserts or updates a linear memory snapshot inside the database.
func (s *PostgresSnapshotStore) Save(id string, snapshot []byte) error {
	query := `
	INSERT INTO snapshots (id, snapshot, updated_at)
	VALUES ($1, $2, CURRENT_TIMESTAMP)
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
func (s *PostgresSnapshotStore) Load(id string) ([]byte, error) {
	query := `SELECT snapshot FROM snapshots WHERE id = $1;`

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
func (s *PostgresSnapshotStore) SaveDeltas(id string, deltas map[int][]byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for SaveDeltas: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
	INSERT INTO memory_deltas (instance_id, page_index, data, updated_at)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
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
func (s *PostgresSnapshotStore) LoadDeltas(id string) (map[int][]byte, error) {
	query := `SELECT page_index, data FROM memory_deltas WHERE instance_id = $1;`
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
func (s *PostgresSnapshotStore) SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error {
	query := `
	INSERT INTO oplog (instance_id, call_index, api_name, request_payload, response_payload, created_at)
	VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
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
func (s *PostgresSnapshotStore) LoadOplog(id string) ([]OplogEntry, error) {
	query := `SELECT call_index, api_name, request_payload, response_payload FROM oplog WHERE instance_id = $1 ORDER BY call_index ASC;`
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
func (s *PostgresSnapshotStore) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("DELETE FROM snapshots WHERE id = $1;", id)
	_, _ = tx.Exec("DELETE FROM memory_deltas WHERE instance_id = $1;", id)
	_, _ = tx.Exec("DELETE FROM oplog WHERE instance_id = $1;", id)
	_, _ = tx.Exec("DELETE FROM instance_meta WHERE instance_id = $1;", id)

	return tx.Commit()
}

// TruncateDeltas deletes all memory deltas for the instance.
func (s *PostgresSnapshotStore) TruncateDeltas(id string) error {
	query := `DELETE FROM memory_deltas WHERE instance_id = $1;`
	_, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to truncate deltas in postgres: %w", err)
	}
	return nil
}

// TruncateOplog deletes all oplog entries for the instance at or below the given call index.
func (s *PostgresSnapshotStore) TruncateOplog(id string, beforeCallIndex int) error {
	query := `DELETE FROM oplog WHERE instance_id = $1 AND call_index <= $2;`
	_, err := s.db.Exec(query, id, beforeCallIndex)
	if err != nil {
		return fmt.Errorf("failed to truncate oplog in postgres: %w", err)
	}
	return nil
}

// LoadMetadata retrieves the instance metadata from Postgres.
func (s *PostgresSnapshotStore) LoadMetadata(id string) (*InstanceMeta, error) {
	query := `SELECT instance_id, wasm_hash, version FROM instance_meta WHERE instance_id = $1;`
	var meta InstanceMeta
	err := s.db.QueryRow(query, id).Scan(&meta.InstanceID, &meta.WasmHash, &meta.Version)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to load metadata from postgres: %w", err)
	}
	return &meta, nil
}

// SaveMetadata saves metadata or atomically updates version via CAS in Postgres.
func (s *PostgresSnapshotStore) SaveMetadata(meta *InstanceMeta) (bool, error) {
	if meta.Version == 0 {
		query := `INSERT INTO instance_meta (instance_id, wasm_hash, version, updated_at) VALUES ($1, $2, 1, CURRENT_TIMESTAMP);`
		_, err := s.db.Exec(query, meta.InstanceID, meta.WasmHash)
		if err != nil {
			return false, nil
		}
		meta.Version = 1
		return true, nil
	}

	query := `UPDATE instance_meta SET version = $1, wasm_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE instance_id = $3 AND version = $4;`
	res, err := s.db.Exec(query, meta.Version+1, meta.WasmHash, meta.InstanceID, meta.Version)
	if err != nil {
		return false, fmt.Errorf("failed to update metadata in postgres: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, nil
	}
	meta.Version++
	return true, nil
}

// SaveWasm saves a WASM module binary by its SHA256 hash in Postgres.
func (s *PostgresSnapshotStore) SaveWasm(hash string, wasmBytes []byte) error {
	query := `INSERT INTO wasm_modules (hash, wasm_bytes) VALUES ($1, $2) ON CONFLICT(hash) DO NOTHING;`
	_, err := s.db.Exec(query, hash, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to save WASM module to postgres %s: %w", hash, err)
	}
	return nil
}

// LoadWasm loads a WASM module binary by its SHA256 hash from Postgres.
func (s *PostgresSnapshotStore) LoadWasm(hash string) ([]byte, error) {
	query := `SELECT wasm_bytes FROM wasm_modules WHERE hash = $1;`
	var wasmBytes []byte
	err := s.db.QueryRow(query, hash).Scan(&wasmBytes)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("wasm module not found in postgres: %s", hash)
	} else if err != nil {
		return nil, fmt.Errorf("failed to load WASM module from postgres %s: %w", hash, err)
	}
	return wasmBytes, nil
}

// Close gracefully closes the Postgres connection.
func (s *PostgresSnapshotStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
