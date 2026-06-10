//go:build !wasm

package wasman

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// PostgresSnapshotStore implements SnapshotStore using a PostgreSQL database.
type PostgresSnapshotStore struct {
	db *sql.DB
}

var _ SnapshotStore = (*PostgresSnapshotStore)(nil)

// NewPostgresSnapshotStore initializes a new PostgreSQL-native SnapshotStore.
func NewPostgresSnapshotStore(db *sql.DB, masterKey string) (*PostgresSnapshotStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection cannot be nil")
	}
	if masterKey != "" {
		keyInitMu.Lock()
		snapshotKey = sha256.Sum256([]byte(masterKey))
		keyInitialized = true
		keyInitMu.Unlock()
	}
	return &PostgresSnapshotStore{
		db: db,
	}, nil
}

// Save writes a full memory snapshot to the database.
func (s *PostgresSnapshotStore) Save(id string, snapshot []byte) error {
	compressed, err := compressData(snapshot)
	if err != nil {
		return fmt.Errorf("failed to compress/encrypt snapshot for '%s': %w", id, err)
	}

	query := `INSERT INTO bpmn_wasm_snapshots (instance_id, snapshot_data, updated_at)
              VALUES ($1, $2, NOW())
              ON CONFLICT (instance_id) DO UPDATE
              SET snapshot_data = EXCLUDED.snapshot_data, updated_at = NOW()`
	_, err = s.db.Exec(query, id, compressed)
	if err != nil {
		return fmt.Errorf("failed to save snapshot for '%s': %w", id, err)
	}
	return nil
}

// Load reads a full memory snapshot from the database.
func (s *PostgresSnapshotStore) Load(id string) ([]byte, error) {
	var compressed []byte
	query := `SELECT snapshot_data FROM bpmn_wasm_snapshots WHERE instance_id = $1`
	err := s.db.QueryRow(query, id).Scan(&compressed)
	if err != nil {
		return nil, err
	}
	return decompressData(compressed)
}

// Delete removes all data associated with the instance from the database snapshot tables.
func (s *PostgresSnapshotStore) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.Exec(`DELETE FROM bpmn_wasm_snapshots WHERE instance_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM bpmn_wasm_deltas WHERE instance_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM bpmn_wasm_oplog WHERE instance_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM bpmn_wasm_metadata WHERE instance_id = $1`, id)
	_, _ = tx.Exec(`DELETE FROM bpmn_wasm_active_index WHERE instance_id = $1`, id)

	return tx.Commit()
}

// SaveDeltas saves memory deltas to the database.
func (s *PostgresSnapshotStore) SaveDeltas(id string, deltas map[int][]byte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO bpmn_wasm_deltas (instance_id, page_index, delta_data)
              VALUES ($1, $2, $3)
              ON CONFLICT (instance_id, page_index) DO UPDATE
              SET delta_data = EXCLUDED.delta_data`

	for pageIdx, delta := range deltas {
		compressed, err := compressData(delta)
		if err != nil {
			return err
		}
		_, err = tx.Exec(query, id, pageIdx, compressed)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadDeltas retrieves memory deltas from the database.
func (s *PostgresSnapshotStore) LoadDeltas(id string) (map[int][]byte, error) {
	rows, err := s.db.Query(`SELECT page_index, delta_data FROM bpmn_wasm_deltas WHERE instance_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[int][]byte)
	for rows.Next() {
		var pageIdx int
		var compressed []byte
		if err := rows.Scan(&pageIdx, &compressed); err != nil {
			return nil, err
		}
		decompressed, err := decompressData(compressed)
		if err != nil {
			return nil, err
		}
		res[pageIdx] = decompressed
	}
	return res, nil
}

// TruncateDeltas deletes memory deltas for the instance from the database.
func (s *PostgresSnapshotStore) TruncateDeltas(id string) error {
	_, err := s.db.Exec(`DELETE FROM bpmn_wasm_deltas WHERE instance_id = $1`, id)
	return err
}

// SaveOplog appends an API call to the database oplog table.
func (s *PostgresSnapshotStore) SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error {
	encRequest, err := compressData(request)
	if err != nil {
		return err
	}
	encResponse, err := compressData(response)
	if err != nil {
		return err
	}

	query := `INSERT INTO bpmn_wasm_oplog (instance_id, call_index, api_name, request_payload, response_payload)
              VALUES ($1, $2, $3, $4, $5)
              ON CONFLICT (instance_id, call_index) DO UPDATE
              SET api_name = EXCLUDED.api_name, request_payload = EXCLUDED.request_payload, response_payload = EXCLUDED.response_payload`
	_, err = s.db.Exec(query, id, callIndex, apiName, encRequest, encResponse)
	return err
}

// LoadOplog retrieves the oplog entries from the database.
func (s *PostgresSnapshotStore) LoadOplog(id string) ([]OplogEntry, error) {
	rows, err := s.db.Query(`SELECT call_index, api_name, request_payload, response_payload FROM bpmn_wasm_oplog WHERE instance_id = $1 ORDER BY call_index ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OplogEntry
	for rows.Next() {
		var entry OplogEntry
		var encRequest, encResponse []byte
		if err := rows.Scan(&entry.CallIndex, &entry.ApiName, &encRequest, &encResponse); err != nil {
			return nil, err
		}
		decRequest, err := decompressData(encRequest)
		if err != nil {
			return nil, err
		}
		decResponse, err := decompressData(encResponse)
		if err != nil {
			return nil, err
		}
		entry.RequestPayload = decRequest
		entry.ResponsePayload = decResponse
		list = append(list, entry)
	}
	return list, nil
}

// TruncateOplog deletes oplog entries at or below the given call index.
func (s *PostgresSnapshotStore) TruncateOplog(id string, beforeCallIndex int) error {
	_, err := s.db.Exec(`DELETE FROM bpmn_wasm_oplog WHERE instance_id = $1 AND call_index <= $2`, id, beforeCallIndex)
	return err
}

// SaveMetadata saves metadata or atomically updates version via CAS.
func (s *PostgresSnapshotStore) SaveMetadata(meta *InstanceMeta) (bool, error) {
	nextVersion := meta.Version + 1
	if meta.Version == 0 {
		nextVersion = 1
	}

	tempMeta := *meta
	tempMeta.Version = nextVersion
	tempMeta.ETag = ""

	jsonData, err := json.Marshal(tempMeta)
	if err != nil {
		return false, err
	}

	encryptedBytes, err := compressData(jsonData)
	if err != nil {
		return false, err
	}

	var res sql.Result
	if meta.Version == 0 {
		query := `INSERT INTO bpmn_wasm_metadata (instance_id, version, metadata_bytes, updated_at)
                  VALUES ($1, $2, $3, NOW())`
		res, err = s.db.Exec(query, meta.InstanceID, nextVersion, encryptedBytes)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "violates unique constraint") {
				return false, nil
			}
			return false, err
		}
	} else {
		query := `UPDATE bpmn_wasm_metadata
                  SET version = $1, metadata_bytes = $2, updated_at = NOW()
                  WHERE instance_id = $3 AND version = $4`
		res, err = s.db.Exec(query, nextVersion, encryptedBytes, meta.InstanceID, meta.Version)
		if err != nil {
			return false, err
		}
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows == 0 {
		return false, nil
	}

	meta.Version = nextVersion
	meta.ETag = fmt.Sprintf("%d", nextVersion)
	return true, nil
}

// LoadMetadata retrieves the instance metadata from the database.
func (s *PostgresSnapshotStore) LoadMetadata(id string) (*InstanceMeta, error) {
	var version int
	var encryptedBytes []byte
	query := `SELECT version, metadata_bytes FROM bpmn_wasm_metadata WHERE instance_id = $1`
	err := s.db.QueryRow(query, id).Scan(&version, &encryptedBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	jsonData, err := decompressData(encryptedBytes)
	if err != nil {
		return nil, err
	}

	var meta InstanceMeta
	if err := json.Unmarshal(jsonData, &meta); err != nil {
		return nil, err
	}

	meta.Version = version
	meta.ETag = fmt.Sprintf("%d", version)
	return &meta, nil
}

// SaveWasm saves a compiled WASM module binary to the database.
func (s *PostgresSnapshotStore) SaveWasm(hash string, wasmBytes []byte) error {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM bpmn_wasm_registry WHERE hash = $1)`, hash).Scan(&exists)
	if err == nil && exists {
		return nil
	}

	compressed, err := compressData(wasmBytes)
	if err != nil {
		return err
	}

	query := `INSERT INTO bpmn_wasm_registry (hash, wasm_bytes, created_at)
              VALUES ($1, $2, NOW())
              ON CONFLICT (hash) DO NOTHING`
	_, err = s.db.Exec(query, hash, compressed)
	return err
}

// LoadWasm loads a compiled WASM module binary from the database.
func (s *PostgresSnapshotStore) LoadWasm(hash string) ([]byte, error) {
	var compressed []byte
	query := `SELECT wasm_bytes FROM bpmn_wasm_registry WHERE hash = $1`
	err := s.db.QueryRow(query, hash).Scan(&compressed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("wasm module not found: %s", hash)
		}
		return nil, err
	}
	return decompressData(compressed)
}

// UpdateActiveIndex updates the active instance status in the database.
func (s *PostgresSnapshotStore) UpdateActiveIndex(id string, info []byte, completed bool) error {
	if completed {
		_, err := s.db.Exec(`DELETE FROM bpmn_wasm_active_index WHERE instance_id = $1`, id)
		return err
	}

	query := `INSERT INTO bpmn_wasm_active_index (instance_id, info, updated_at)
              VALUES ($1, $2, NOW())
              ON CONFLICT (instance_id) DO UPDATE
              SET info = EXCLUDED.info, updated_at = NOW()`
	_, err := s.db.Exec(query, id, info)
	return err
}

// LoadActiveIndex compiles the active index list from the database.
func (s *PostgresSnapshotStore) LoadActiveIndex() ([]byte, error) {
	rows, err := s.db.Query(`SELECT info FROM bpmn_wasm_active_index`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activeList []json.RawMessage
	for rows.Next() {
		var info []byte
		if err := rows.Scan(&info); err != nil {
			return nil, err
		}
		activeList = append(activeList, json.RawMessage(info))
	}

	if len(activeList) == 0 {
		return []byte("[]"), nil
	}

	resultBytes, err := json.Marshal(activeList)
	if err != nil {
		return nil, err
	}
	return resultBytes, nil
}
