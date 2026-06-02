-- atlas:txmode none
CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    snapshot BYTEA NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memory_deltas (
    instance_id TEXT,
    page_index INTEGER,
    data BYTEA NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (instance_id, page_index)
);

CREATE TABLE IF NOT EXISTS oplog (
    instance_id TEXT,
    call_index INTEGER,
    api_name TEXT NOT NULL,
    request_payload BYTEA,
    response_payload BYTEA,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (instance_id, call_index)
);

CREATE TABLE IF NOT EXISTS instance_meta (
    instance_id TEXT PRIMARY KEY,
    wasm_hash TEXT NOT NULL,
    version INTEGER NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wasm_modules (
    hash TEXT PRIMARY KEY,
    wasm_bytes BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
