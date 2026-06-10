package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"github.com/nativebpm/connectors/wasman"
)

const (
	instanceID = "pg-demo-instance"
	serverAddr = "localhost:18081"
)

func main() {
	slog.Info("[HOST] Starting Native Postgres Durable WASM Execution Orchestrator")

	// 1. Read database connection string
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "postgres://demo_user:demo_password@localhost:5435/nativebpm_demo?sslmode=disable"
	}

	ctx := context.Background()

	// 2. Connect to PostgreSQL
	slog.Info("[HOST] Connecting to PostgreSQL", "url", dbURL)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		slog.Error("[HOST] Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 3. Initialize Schema & Tables
	if err := initSchema(db); err != nil {
		slog.Error("[HOST] Failed to initialize database schema", "error", err)
		os.Exit(1)
	}
	slog.Info("[HOST] Database schema initialized and verified successfully.")

	// Pre-insert parent records for foreign key constraints
	if err := insertDummyInstance(db); err != nil {
		slog.Error("[HOST] Failed to insert dummy definition/instance", "error", err)
		os.Exit(1)
	}

	// 4. Initialize Postgres snapshot store
	masterKey := os.Getenv("CRYPTENV_KEY")
	if masterKey == "" {
		masterKey = "my-secret-key-for-testing-purposes-only-2026"
		os.Setenv("CRYPTENV_KEY", masterKey)
	}

	store, err := wasman.NewPostgresSnapshotStore(db, masterKey)
	if err != nil {
		slog.Error("[HOST] Failed to initialize Postgres store", "error", err)
		os.Exit(1)
	}

	// 5. Start local Mock HTTP Server to mock external REST calls
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(ctx)

	// Give the server a small moment to bind to the port
	time.Sleep(100 * time.Millisecond)

	// 6. Initialize the Reusable Durable WASM Engine with Postgres store
	wasmPath := os.Getenv("WASM_PATH")
	if wasmPath == "" {
		wasmPath = filepath.Join("..", "worker", "worker.wasm")
	}

	// Clear any leftover snapshot from previous runs
	_ = store.Delete(ctx, instanceID)

	// 7. RUN 1: Execute with simulated crash on the first checkpoint
	slog.Info("[HOST] RUN 1: Executing WASM from scratch with simulated crash")
	crashed, err := wasman.NewTestRunner().
		WithWasmPath(wasmPath).
		WithStore(store).
		WithSessionID(instanceID).
		WithServer(serverAddr).
		WithCrash(true).
		Run()
	if err != nil {
		if crashed {
			slog.Info("[HOST] Execution successfully suspended/crashed", "error", err)
		} else {
			slog.Error("[HOST] Execution failed", "error", err)
			os.Exit(1)
		}
	}

	// Verify snapshot exists in Postgres
	snapshot, err := store.Load(ctx, instanceID)
	if err != nil || len(snapshot) == 0 {
		slog.Error("[HOST] Snapshot was not found in PostgreSQL", "error", err)
		os.Exit(1)
	}
	slog.Info("[HOST] Verified that snapshot was successfully written to bpmn_wasm_snapshots table")

	// 8. RUN 2: Restore from checkpoint and resume execution
	slog.Info("[HOST] RUN 2: Restoring from PostgreSQL snapshot and completing execution")
	crashed, err = wasman.NewTestRunner().
		WithWasmPath(wasmPath).
		WithStore(store).
		WithSessionID(instanceID).
		WithServer(serverAddr).
		WithCrash(false).
		Run()
	if err != nil {
		slog.Error("[HOST] Resumed execution failed", "error", err)
		os.Exit(1)
	}

	if crashed {
		slog.Error("[HOST] Resumed execution crashed unexpectedly!")
		os.Exit(1)
	}

	slog.Info("[HOST] Durable WASM Execution on Postgres Snapshot Store completed successfully!")
	os.Exit(0)
}

func initSchema(db *sql.DB) error {
	// Re-create the schema
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
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	// Create instances partitions
	for i := 0; i < 16; i++ {
		_, _ = db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS bpmn_instances_%d PARTITION OF bpmn_instances FOR VALUES WITH (MODULUS 16, REMAINDER %d);", i, i))
	}

	wasmRegistryQuery := `CREATE TABLE IF NOT EXISTS bpmn_wasm_registry (
		hash VARCHAR PRIMARY KEY,
		wasm_bytes BYTEA NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`
	if _, err := db.Exec(wasmRegistryQuery); err != nil {
		return err
	}

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
		if _, err := db.Exec(pt.schema); err != nil {
			return err
		}
		for i := 0; i < 16; i++ {
			partitionQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s_%d PARTITION OF %s FOR VALUES WITH (MODULUS 16, REMAINDER %d);", pt.name, i, pt.name, i)
			_, _ = db.Exec(partitionQuery)
		}
	}

	return nil
}

func insertDummyInstance(db *sql.DB) error {
	defQuery := `INSERT INTO bpmn_definitions (hash, id, name, xml_data, json_data, deployed_at)
                 VALUES ($1, $2, $3, $4, $5, NOW()) ON CONFLICT DO NOTHING`
	if _, err := db.Exec(defQuery, "def-hash-123", "proc-123", "Test Process", []byte("<xml>"), []byte("{}")); err != nil {
		return err
	}

	instQuery := `INSERT INTO bpmn_instances (id, process_id, definition_hash, business_key, state, version, completed, updated_at, tenant_id)
                  VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8) ON CONFLICT DO NOTHING`
	if _, err := db.Exec(instQuery, instanceID, "proc-123", "def-hash-123", "bkey-123", []byte("{}"), 1, false, "default"); err != nil {
		return err
	}

	return nil
}

func startMockServer(addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		// Stream 40KB of lowercase text
		line := []byte("durable execution engine based on webassembly and tinygo native postgres store test line.\n")
		for i := 0; i < 500; i++ {
			_, _ = w.Write(line)
		}
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		totalBytes := 0
		allUppercase := true

		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				totalBytes += n
				for i := 0; i < n; i++ {
					if buf[i] >= 'a' && buf[i] <= 'z' {
						allUppercase = false
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		slog.Info("[MOCK SERVER] Received payload", "bytes", totalBytes, "all_uppercase", allUppercase)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			slog.Error("[MOCK SERVER] Failed to listen", "error", err)
			return
		}
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			slog.Error("[MOCK SERVER] Serve error", "error", err)
		}
	}()

	slog.Info("[MOCK SERVER] Listening", "addr", "http://"+addr)
	return server
}
