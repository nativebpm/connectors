-- name: SaveSnapshot
INSERT INTO snapshots (id, snapshot, updated_at)
VALUES ($1, $2, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
    snapshot = excluded.snapshot,
    updated_at = CURRENT_TIMESTAMP;

-- name: LoadSnapshot
SELECT snapshot FROM snapshots WHERE id = $1;

-- name: SaveDeltas
INSERT INTO memory_deltas (instance_id, page_index, data, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
ON CONFLICT(instance_id, page_index) DO UPDATE SET
    data = excluded.data,
    updated_at = CURRENT_TIMESTAMP;

-- name: LoadDeltas
SELECT page_index, data FROM memory_deltas WHERE instance_id = $1;

-- name: TruncateDeltas
DELETE FROM memory_deltas WHERE instance_id = $1;

-- name: SaveOplog
INSERT INTO oplog (instance_id, call_index, api_name, request_payload, response_payload, created_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
ON CONFLICT(instance_id, call_index) DO UPDATE SET
    api_name = excluded.api_name,
    request_payload = excluded.request_payload,
    response_payload = excluded.response_payload;

-- name: LoadOplog
SELECT call_index, api_name, request_payload, response_payload FROM oplog WHERE instance_id = $1 ORDER BY call_index ASC;

-- name: TruncateOplog
DELETE FROM oplog WHERE instance_id = $1 AND call_index <= $2;

-- name: LoadMetadata
SELECT instance_id, wasm_hash, version FROM instance_meta WHERE instance_id = $1;

-- name: SaveMetadataInsert
INSERT INTO instance_meta (instance_id, wasm_hash, version, updated_at) VALUES ($1, $2, 1, CURRENT_TIMESTAMP);

-- name: SaveMetadataUpdate
UPDATE instance_meta SET version = $1, wasm_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE instance_id = $3 AND version = $4;

-- name: SaveWasm
INSERT INTO wasm_modules (hash, wasm_bytes) VALUES ($1, $2) ON CONFLICT(hash) DO NOTHING;

-- name: LoadWasm
SELECT wasm_bytes FROM wasm_modules WHERE hash = $1;

-- name: DeleteSnapshots
DELETE FROM snapshots WHERE id = $1;

-- name: DeleteDeltas
DELETE FROM memory_deltas WHERE instance_id = $1;

-- name: DeleteOplog
DELETE FROM oplog WHERE instance_id = $1;

-- name: DeleteMeta
DELETE FROM instance_meta WHERE instance_id = $1;
