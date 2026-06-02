-- name: SaveSnapshot
INSERT INTO snapshots (id, snapshot, updated_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
    snapshot = excluded.snapshot,
    updated_at = CURRENT_TIMESTAMP;

-- name: LoadSnapshot
SELECT snapshot FROM snapshots WHERE id = ?;

-- name: SaveDeltas
INSERT INTO memory_deltas (instance_id, page_index, data, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(instance_id, page_index) DO UPDATE SET
    data = excluded.data,
    updated_at = CURRENT_TIMESTAMP;

-- name: LoadDeltas
SELECT page_index, data FROM memory_deltas WHERE instance_id = ?;

-- name: TruncateDeltas
DELETE FROM memory_deltas WHERE instance_id = ?;

-- name: SaveOplog
INSERT INTO oplog (instance_id, call_index, api_name, request_payload, response_payload, created_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(instance_id, call_index) DO UPDATE SET
    api_name = excluded.api_name,
    request_payload = excluded.request_payload,
    response_payload = excluded.response_payload;

-- name: LoadOplog
SELECT call_index, api_name, request_payload, response_payload FROM oplog WHERE instance_id = ? ORDER BY call_index ASC;

-- name: TruncateOplog
DELETE FROM oplog WHERE instance_id = ? AND call_index <= ?;

-- name: LoadMetadata
SELECT instance_id, wasm_hash, version FROM instance_meta WHERE instance_id = ?;

-- name: SaveMetadataInsert
INSERT INTO instance_meta (instance_id, wasm_hash, version, updated_at) VALUES (?, ?, 1, CURRENT_TIMESTAMP);

-- name: SaveMetadataUpdate
UPDATE instance_meta SET version = ?, wasm_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE instance_id = ? AND version = ?;

-- name: SaveWasm
INSERT OR IGNORE INTO wasm_modules (hash, wasm_bytes) VALUES (?, ?);

-- name: LoadWasm
SELECT wasm_bytes FROM wasm_modules WHERE hash = ?;

-- name: DeleteSnapshots
DELETE FROM snapshots WHERE id = ?;

-- name: DeleteDeltas
DELETE FROM memory_deltas WHERE instance_id = ?;

-- name: DeleteOplog
DELETE FROM oplog WHERE instance_id = ?;

-- name: DeleteMeta
DELETE FROM instance_meta WHERE instance_id = ?;
