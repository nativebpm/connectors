//go:build !wasm

package wasman

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestDb(t *testing.T) (*sql.DB, string) {
	dbURL := os.Getenv("TEST_POSTGRES_URL")
	if dbURL == "" {
		// Fallback to check if a generic DB_URL is present, or skip
		dbURL = os.Getenv("DB_URL")
		if dbURL == "" {
			t.Skip("Skipping PostgresSnapshotStore integration tests: TEST_POSTGRES_URL/DB_URL is not set")
		}
	}

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)

	// Clean up existing tables to ensure partitioning structure is applied correctly
	_, _ = db.Exec("DROP TABLE IF EXISTS bpmn_wasm_snapshots, bpmn_wasm_deltas, bpmn_wasm_oplog, bpmn_wasm_metadata, bpmn_wasm_active_index, bpmn_wasm_registry CASCADE;")
	_, _ = db.Exec("DROP TABLE IF EXISTS bpmn_instances CASCADE;")
	_, _ = db.Exec("DROP TABLE IF EXISTS bpmn_definitions CASCADE;")

	// Ensure the parent tables and the new WASM tables exist
	// In the real application, schema.sql handles this. For the test, we execute them explicitly.
	schemaQueries := []string{
		`CREATE TABLE IF NOT EXISTS bpmn_definitions (
			hash VARCHAR PRIMARY KEY,
			id VARCHAR NOT NULL,
			name VARCHAR NOT NULL,
			xml_data BYTEA,
			json_data JSONB,
			deployed_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS bpmn_instances (
			id VARCHAR,
			process_id VARCHAR NOT NULL,
			definition_hash VARCHAR NOT NULL REFERENCES bpmn_definitions(hash),
			business_key VARCHAR,
			state JSONB NOT NULL,
			version INT NOT NULL,
			completed BOOLEAN NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			tenant_id VARCHAR NOT NULL DEFAULT 'default',
			PRIMARY KEY (id)
		) PARTITION BY HASH (id);`,
	}

	for _, q := range schemaQueries {
		_, err := db.Exec(q)
		require.NoError(t, err, "failed executing query: %s", q)
	}

	// Create instances partitions
	for i := 0; i < 16; i++ {
		_, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS bpmn_instances_%d PARTITION OF bpmn_instances FOR VALUES WITH (MODULUS 16, REMAINDER %d);", i, i))
		require.NoError(t, err)
	}

	wasmRegistryQuery := `CREATE TABLE IF NOT EXISTS bpmn_wasm_registry (
		hash VARCHAR PRIMARY KEY,
		wasm_bytes BYTEA NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`
	_, err = db.Exec(wasmRegistryQuery)
	require.NoError(t, err)

	partitionedTables := []struct {
		name   string
		schema string
	}{
		{
			name: "bpmn_wasm_metadata",
			schema: `CREATE TABLE IF NOT EXISTS bpmn_wasm_metadata (
				instance_id VARCHAR REFERENCES bpmn_instances(id) ON DELETE CASCADE,
				version INT NOT NULL,
				metadata_bytes BYTEA NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (instance_id)
			) PARTITION BY HASH (instance_id);`,
		},
		{
			name: "bpmn_wasm_snapshots",
			schema: `CREATE TABLE IF NOT EXISTS bpmn_wasm_snapshots (
				instance_id VARCHAR REFERENCES bpmn_instances(id) ON DELETE CASCADE,
				snapshot_data BYTEA NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (instance_id)
			) PARTITION BY HASH (instance_id);`,
		},
		{
			name: "bpmn_wasm_deltas",
			schema: `CREATE TABLE IF NOT EXISTS bpmn_wasm_deltas (
				instance_id VARCHAR REFERENCES bpmn_instances(id) ON DELETE CASCADE,
				page_index INT,
				delta_data BYTEA NOT NULL,
				PRIMARY KEY (instance_id, page_index)
			) PARTITION BY HASH (instance_id);`,
		},
		{
			name: "bpmn_wasm_oplog",
			schema: `CREATE TABLE IF NOT EXISTS bpmn_wasm_oplog (
				instance_id VARCHAR REFERENCES bpmn_instances(id) ON DELETE CASCADE,
				call_index INT,
				api_name VARCHAR NOT NULL,
				request_payload BYTEA NOT NULL,
				response_payload BYTEA NOT NULL,
				PRIMARY KEY (instance_id, call_index)
			) PARTITION BY HASH (instance_id);`,
		},
		{
			name: "bpmn_wasm_active_index",
			schema: `CREATE TABLE IF NOT EXISTS bpmn_wasm_active_index (
				instance_id VARCHAR REFERENCES bpmn_instances(id) ON DELETE CASCADE,
				info BYTEA NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (instance_id)
			) PARTITION BY HASH (instance_id);`,
		},
	}

	for _, pt := range partitionedTables {
		_, err := db.Exec(pt.schema)
		require.NoError(t, err, "failed creating partitioned table schema: %s", pt.name)

		for i := 0; i < 16; i++ {
			partitionQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s_%d PARTITION OF %s FOR VALUES WITH (MODULUS 16, REMAINDER %d);", pt.name, i, pt.name, i)
			_, err := db.Exec(partitionQuery)
			require.NoError(t, err, "failed creating partition: %s_%d", pt.name, i)
		}
	}

	// Clean up snapshot and instances tables for clean test execution
	cleanupQueries := []string{
		"TRUNCATE TABLE bpmn_wasm_snapshots CASCADE",
		"TRUNCATE TABLE bpmn_wasm_deltas CASCADE",
		"TRUNCATE TABLE bpmn_wasm_oplog CASCADE",
		"TRUNCATE TABLE bpmn_wasm_metadata CASCADE",
		"TRUNCATE TABLE bpmn_wasm_active_index CASCADE",
		"TRUNCATE TABLE bpmn_instances CASCADE",
		"TRUNCATE TABLE bpmn_definitions CASCADE",
	}
	for _, q := range cleanupQueries {
		_, _ = db.Exec(q)
	}

	return db, dbURL
}

func insertDummyDefinitionAndInstance(t *testing.T, db *sql.DB, instanceID string) {
	// We need to create a parent bpmn_instances row because of FOREIGN KEY constraints.
	defQuery := `INSERT INTO bpmn_definitions (hash, id, name, xml_data, json_data, deployed_at)
                 VALUES ($1, $2, $3, $4, $5, NOW()) ON CONFLICT DO NOTHING`
	_, err := db.Exec(defQuery, "def-hash-123", "proc-123", "Test Process", []byte("<xml>"), []byte("{}"))
	require.NoError(t, err)

	instQuery := `INSERT INTO bpmn_instances (id, process_id, definition_hash, business_key, state, version, completed, updated_at, tenant_id)
                  VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8) ON CONFLICT DO NOTHING`
	_, err = db.Exec(instQuery, instanceID, "proc-123", "def-hash-123", "bkey-123", []byte("{}"), 1, false, "default")
	require.NoError(t, err)
}

func TestPostgresSnapshotStore_Lifecycle(t *testing.T) {
	db, _ := initTestDb(t)
	defer db.Close()

	masterKey := "my-secret-key-for-testing-purposes-only-2026"
	store, err := NewPostgresSnapshotStore(db, masterKey)
	require.NoError(t, err)

	instanceID := "test-instance-123"
	insertDummyDefinitionAndInstance(t, db, instanceID)
	ctx := context.Background()

	// 1. Test Save & Load Snapshot
	snapData := []byte("full-binary-snapshot-state-data-content")
	err = store.Save(ctx, instanceID, snapData)
	require.NoError(t, err)

	loadedSnap, err := store.Load(ctx, instanceID)
	require.NoError(t, err)
	assert.Equal(t, snapData, loadedSnap)

	// Verify encryption and compression in DB
	var rawDbBytes []byte
	err = db.QueryRow("SELECT snapshot_data FROM bpmn_wasm_snapshots WHERE instance_id = $1", instanceID).Scan(&rawDbBytes)
	require.NoError(t, err)
	assert.NotEqual(t, snapData, rawDbBytes, "Raw snapshot bytes in DB must be encrypted/compressed, not raw plaintext")

	// 2. Test Save & Load Deltas
	deltas := map[int][]byte{
		0: []byte("delta-page-0-bytes"),
		4: []byte("delta-page-4-bytes"),
	}
	err = store.SaveDeltas(ctx, instanceID, deltas)
	require.NoError(t, err)

	loadedDeltas, err := store.LoadDeltas(ctx, instanceID)
	require.NoError(t, err)
	assert.Len(t, loadedDeltas, 2)
	assert.Equal(t, []byte("delta-page-0-bytes"), loadedDeltas[0])
	assert.Equal(t, []byte("delta-page-4-bytes"), loadedDeltas[4])

	// Verify encryption of delta data in DB
	var rawDeltaBytes []byte
	err = db.QueryRow("SELECT delta_data FROM bpmn_wasm_deltas WHERE instance_id = $1 AND page_index = 0", instanceID).Scan(&rawDeltaBytes)
	require.NoError(t, err)
	assert.NotEqual(t, []byte("delta-page-0-bytes"), rawDeltaBytes, "Raw delta bytes in DB must be encrypted/compressed")

	// 3. Test Save & Load Oplog
	err = store.SaveOplog(ctx, instanceID, 1, "test_call", []byte("request-data"), []byte("response-data"))
	require.NoError(t, err)

	oplog, err := store.LoadOplog(ctx, instanceID)
	require.NoError(t, err)
	require.Len(t, oplog, 1)
	assert.Equal(t, 1, oplog[0].CallIndex)
	assert.Equal(t, "test_call", oplog[0].ApiName)
	assert.Equal(t, []byte("request-data"), oplog[0].RequestPayload)
	assert.Equal(t, []byte("response-data"), oplog[0].ResponsePayload)

	// Verify encryption of oplog payload in DB
	var rawReqBytes []byte
	err = db.QueryRow("SELECT request_payload FROM bpmn_wasm_oplog WHERE instance_id = $1 AND call_index = 1", instanceID).Scan(&rawReqBytes)
	require.NoError(t, err)
	assert.NotEqual(t, []byte("request-data"), rawReqBytes, "Oplog request payload must be encrypted")

	// 4. Test Metadata OCC Fencing
	meta := &InstanceMeta{
		InstanceID:     instanceID,
		WasmHash:       "wasm-hash-999",
		Version:        0,
		ProcessID:      "proc-123",
		DefinitionHash: "def-hash-123",
		BusinessKey:    "bkey-123",
		BpmnState:      []byte("bpmn-state-bytes"),
		Completed:      false,
	}

	// First insert
	ok, err := store.SaveMetadata(ctx, meta)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, meta.Version)
	assert.Equal(t, "1", meta.ETag)

	// Try to insert again with Version=0 (should fail OCC)
	metaDup := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "wasm-hash-dup",
		Version:    0,
	}
	ok, err = store.SaveMetadata(ctx, metaDup)
	require.NoError(t, err)
	assert.False(t, ok)

	// Load metadata and check values
	loadedMeta, err := store.LoadMetadata(ctx, instanceID)
	require.NoError(t, err)
	require.NotNil(t, loadedMeta)
	assert.Equal(t, 1, loadedMeta.Version)
	assert.Equal(t, "wasm-hash-999", loadedMeta.WasmHash)
	assert.Equal(t, []byte("bpmn-state-bytes"), loadedMeta.BpmnState)

	// Normal update
	loadedMeta.WasmHash = "wasm-hash-999-updated"
	ok, err = store.SaveMetadata(ctx, loadedMeta)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 2, loadedMeta.Version)

	// Stale update (using old meta version)
	meta.WasmHash = "wasm-hash-stale"
	ok, err = store.SaveMetadata(ctx, meta) // meta has version 1
	require.NoError(t, err)
	assert.False(t, ok)

	// Verify final metadata state
	finalMeta, err := store.LoadMetadata(ctx, instanceID)
	require.NoError(t, err)
	assert.Equal(t, 2, finalMeta.Version)
	assert.Equal(t, "wasm-hash-999-updated", finalMeta.WasmHash)

	// 5. Test WASM Registry
	wasmHash := "wasm-sha256-hash-value"
	wasmBytes := []byte("compiled-wasm-binary-mock-bytes")
	err = store.SaveWasm(ctx, wasmHash, wasmBytes)
	require.NoError(t, err)

	loadedWasm, err := store.LoadWasm(ctx, wasmHash)
	require.NoError(t, err)
	assert.Equal(t, wasmBytes, loadedWasm)

	// Verify encryption of WASM registry in DB
	var rawWasmBytes []byte
	err = db.QueryRow("SELECT wasm_bytes FROM bpmn_wasm_registry WHERE hash = $1", wasmHash).Scan(&rawWasmBytes)
	require.NoError(t, err)
	assert.NotEqual(t, wasmBytes, rawWasmBytes, "Raw WASM bytes in DB must be encrypted/compressed")

	// 6. Test Active Index
	info := map[string]interface{}{
		"instance_id": instanceID,
		"process_id":  "proc-123",
		"completed":   false,
	}
	infoBytes, err := json.Marshal(info)
	require.NoError(t, err)

	err = store.UpdateActiveIndex(ctx, instanceID, infoBytes, false)
	require.NoError(t, err)

	activeIndexBytes, err := store.LoadActiveIndex(ctx)
	require.NoError(t, err)

	var activeList []map[string]interface{}
	err = json.Unmarshal(activeIndexBytes, &activeList)
	require.NoError(t, err)
	require.Len(t, activeList, 1)
	assert.Equal(t, instanceID, activeList[0]["instance_id"])
	assert.Equal(t, "proc-123", activeList[0]["process_id"])
	assert.Equal(t, false, activeList[0]["completed"])

	// Test Active Index Completed (removal)
	err = store.UpdateActiveIndex(ctx, instanceID, infoBytes, true)
	require.NoError(t, err)

	activeIndexBytes2, err := store.LoadActiveIndex(ctx)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(activeIndexBytes2))

	// 7. Test Truncations & Delete
	// Re-add active index and deltas/oplog to check delete cascade
	err = store.UpdateActiveIndex(ctx, instanceID, infoBytes, false)
	require.NoError(t, err)

	err = store.TruncateDeltas(ctx, instanceID)
	require.NoError(t, err)
	loadedDeltasTrunc, err := store.LoadDeltas(ctx, instanceID)
	require.NoError(t, err)
	assert.Empty(t, loadedDeltasTrunc)

	err = store.TruncateOplog(ctx, instanceID, 1)
	require.NoError(t, err)
	loadedOplogTrunc, err := store.LoadOplog(ctx, instanceID)
	require.NoError(t, err)
	assert.Empty(t, loadedOplogTrunc)

	// Explicit delete from store
	err = store.Delete(ctx, instanceID)
	require.NoError(t, err)

	// Check all tables are empty for this instanceID
	var exists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM bpmn_wasm_snapshots WHERE instance_id = $1)", instanceID).Scan(&exists)
	assert.False(t, exists)
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM bpmn_wasm_metadata WHERE instance_id = $1)", instanceID).Scan(&exists)
	assert.False(t, exists)
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM bpmn_wasm_active_index WHERE instance_id = $1)", instanceID).Scan(&exists)
	assert.False(t, exists)
}

func BenchmarkPostgresSnapshotStore_EncryptCompress(b *testing.B) {
	// We don't actually need an active DB connection for pure CPU/Memory benchmark of helper methods, but we need NewPostgresSnapshotStore.
	db, _ := sql.Open("postgres", "postgres://nativebpm:password@localhost:5432/nativebpm?sslmode=disable")
	defer db.Close()
	_, err := NewPostgresSnapshotStore(db, "my-secret-key-for-testing-purposes-only-2026")
	if err != nil {
		b.Fatal(err)
	}

	// Simulate 1 MB linear memory
	data := make([]byte, 1024*1024)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i % 256) // predictable data for stable compression ratio
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressData(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostgresSnapshotStore_DecryptDecompress(b *testing.B) {
	db, _ := sql.Open("postgres", "postgres://nativebpm:password@localhost:5432/nativebpm?sslmode=disable")
	defer db.Close()
	_, err := NewPostgresSnapshotStore(db, "my-secret-key-for-testing-purposes-only-2026")
	if err != nil {
		b.Fatal(err)
	}

	// Simulate 1 MB linear memory
	data := make([]byte, 1024*1024)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i % 256)
	}

	encrypted, err := compressData(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decompressData(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPostgresSnapshotStore_SoakStress(t *testing.T) {
	db, _ := initTestDb(t)
	defer db.Close()

	wasmBytes := loadTestWasm(t, "soak_stress")

	tempDir := t.TempDir()
	wasmPath := filepath.Join(tempDir, "test.wasm")
	err := os.WriteFile(wasmPath, wasmBytes, 0644)
	require.NoError(t, err)

	masterKey := "my-secret-key-for-testing-purposes-only-2026"
	store, err := NewPostgresSnapshotStore(db, masterKey)
	require.NoError(t, err)

	const concurrency = 20
	const iterations = 10 // 200 total runs

	// Pre-create parent records in bpmn_instances because of foreign key constraint
	for i := 0; i < concurrency; i++ {
		for j := 0; j < iterations; j++ {
			instanceID := "stress-instance-" + strconv.Itoa(i) + "-" + strconv.Itoa(j)
			insertDummyDefinitionAndInstance(t, db, instanceID)
		}
	}

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				instanceID := "stress-instance-" + strconv.Itoa(workerID) + "-" + strconv.Itoa(j)
				_, err := NewTestRunner().
					WithWasmPath(wasmPath).
					WithStore(store).
					WithSessionID(instanceID).
					WithEntrypoint("run_test").
					WithServer("localhost:0").
					WithCrash(false).
					Run()
				if err != nil {
					t.Errorf("Stress run failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

