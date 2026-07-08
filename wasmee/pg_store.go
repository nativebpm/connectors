package wasmee

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// PostgresSnapshotStore implements wasmee.SnapshotStore using PostgreSQL database.
// It persists WASM memory snapshots, deltas, and oplogs to survive server crashes.
//
// Schema requirements:
//  - bpmn_wasm_snapshots (instance_id, snapshot_data, updated_at)
//  - bpmn_wasm_deltas (instance_id, page_index, delta_data)
//  - bpmn_wasm_oplog (instance_id, call_index, api_name, request_payload, response_payload)
//  - bpmn_wasm_metadata (instance_id, wasm_hash, version, completed, metadata)
type PostgresSnapshotStore struct {
	db *sql.DB
}

var _ SnapshotStore = (*PostgresSnapshotStore)(nil)

// NewPostgresSnapshotStore creates a new PostgreSQL-backed SnapshotStore.
func NewPostgresSnapshotStore(db *sql.DB) *PostgresSnapshotStore {
	return &PostgresSnapshotStore{db: db}
}

func (s *PostgresSnapshotStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bpmn_wasm_snapshots (instance_id, snapshot_data, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (instance_id) DO UPDATE
		   SET snapshot_data = EXCLUDED.snapshot_data, updated_at = NOW()`,
		id, snapshot)
	if err != nil {
		return fmt.Errorf("wasmee pg_store SaveSnapshot %q: %w", id, err)
	}
	return nil
}

func (s *PostgresSnapshotStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT snapshot_data FROM bpmn_wasm_snapshots WHERE instance_id = $1`, id).
		Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wasmee pg_store LoadSnapshot %q: %w", id, err)
	}
	return data, nil
}

func (s *PostgresSnapshotStore) DeleteSnapshot(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("wasmee pg_store DeleteSnapshot tx begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tables := []string{
		"bpmn_wasm_snapshots",
		"bpmn_wasm_deltas",
		"bpmn_wasm_oplog",
		"bpmn_wasm_metadata",
	}

	for _, table := range tables {
		_, _ = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE instance_id = $1`, id)
	}
	return tx.Commit()
}

func (s *PostgresSnapshotStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("wasmee pg_store SaveDeltas tx begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for pageIdx, data := range deltas {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO bpmn_wasm_deltas (instance_id, page_index, delta_data)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (instance_id, page_index) DO UPDATE
			   SET delta_data = EXCLUDED.delta_data`,
			id, pageIdx, data)
		if err != nil {
			return fmt.Errorf("wasmee pg_store SaveDeltas %q page %d: %w", id, pageIdx, err)
		}
	}
	return tx.Commit()
}

func (s *PostgresSnapshotStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT page_index, delta_data FROM bpmn_wasm_deltas WHERE instance_id = $1 ORDER BY page_index`, id)
	if err != nil {
		return nil, fmt.Errorf("wasmee pg_store LoadDeltas %q: %w", id, err)
	}
	defer rows.Close()

	deltas := make(map[int][]byte)
	for rows.Next() {
		var idx int
		var data []byte
		if err := rows.Scan(&idx, &data); err != nil {
			return nil, fmt.Errorf("wasmee pg_store LoadDeltas scan %q: %w", id, err)
		}
		deltas[idx] = data
	}
	if len(deltas) == 0 {
		return nil, nil
	}
	return deltas, rows.Err()
}

func (s *PostgresSnapshotStore) TruncateDeltas(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bpmn_wasm_deltas WHERE instance_id = $1`, id)
	if err != nil {
		return fmt.Errorf("wasmee pg_store TruncateDeltas %q: %w", id, err)
	}
	return nil
}

func (s *PostgresSnapshotStore) SaveOplog(ctx context.Context, id string, entry OplogEntry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bpmn_wasm_oplog (instance_id, call_index, api_name, request_payload, response_payload)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (instance_id, call_index) DO UPDATE
		   SET api_name = EXCLUDED.api_name,
		       request_payload = EXCLUDED.request_payload,
		       response_payload = EXCLUDED.response_payload`,
		id, entry.CallIndex, entry.ApiName, entry.RequestPayload, entry.ResponsePayload)
	if err != nil {
		return fmt.Errorf("wasmee pg_store SaveOplog %q call %d: %w", id, entry.CallIndex, err)
	}
	return nil
}

func (s *PostgresSnapshotStore) LoadOplog(ctx context.Context, id string) ([]OplogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT call_index, api_name, request_payload, response_payload FROM bpmn_wasm_oplog WHERE instance_id = $1 ORDER BY call_index`, id)
	if err != nil {
		return nil, fmt.Errorf("wasmee pg_store LoadOplog %q: %w", id, err)
	}
	defer rows.Close()

	var entries []OplogEntry
	for rows.Next() {
		var e OplogEntry
		if err := rows.Scan(&e.CallIndex, &e.ApiName, &e.RequestPayload, &e.ResponsePayload); err != nil {
			return nil, fmt.Errorf("wasmee pg_store LoadOplog scan %q: %w", id, err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *PostgresSnapshotStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM bpmn_wasm_oplog WHERE instance_id = $1 AND call_index < $2`, id, beforeCallIndex)
	if err != nil {
		return fmt.Errorf("wasmee pg_store TruncateOplog %q before %d: %w", id, beforeCallIndex, err)
	}
	return nil
}

func (s *PostgresSnapshotStore) SaveMetadata(ctx context.Context, meta *InstanceMeta) (bool, error) {
	rawMeta, err := json.Marshal(meta.Metadata)
	if err != nil {
		return false, fmt.Errorf("wasmee pg_store SaveMetadata marshal: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO bpmn_wasm_metadata (instance_id, wasm_hash, version, completed, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (instance_id) DO UPDATE
		   SET wasm_hash = EXCLUDED.wasm_hash,
		       version = EXCLUDED.version,
		       completed = EXCLUDED.completed,
		       metadata = EXCLUDED.metadata`,
		meta.InstanceID, meta.WasmHash, meta.Version, meta.Completed, rawMeta)
	if err != nil {
		return false, fmt.Errorf("wasmee pg_store SaveMetadata %q: %w", meta.InstanceID, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *PostgresSnapshotStore) LoadMetadata(ctx context.Context, id string) (*InstanceMeta, error) {
	var (
		meta    InstanceMeta
		rawMeta []byte
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT instance_id, wasm_hash, version, completed, metadata FROM bpmn_wasm_metadata WHERE instance_id = $1`, id).
		Scan(&meta.InstanceID, &meta.WasmHash, &meta.Version, &meta.Completed, &rawMeta)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wasmee pg_store LoadMetadata %q: %w", id, err)
	}
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &meta.Metadata)
	}
	return &meta, nil
}
