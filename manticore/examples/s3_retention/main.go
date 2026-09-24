package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nativebpm/connectors/manticore"
)

// UniversalS3Mock simulates storing persistent, immutable snapshots in S3/R2.
type UniversalS3Mock struct {
	storage map[string][]byte
}

func (s *UniversalS3Mock) PutSnapshot(instanceID uint64, data []byte) {
	key := fmt.Sprintf("instances/%d/base.bin", instanceID)
	s.storage[key] = data
	fmt.Printf("  [Universal S3] Persisted snapshot: %s (size: %d bytes)\n", key, len(data))
}

func main() {
	s3 := &UniversalS3Mock{storage: make(map[string][]byte)}

	// Configure Manticore as a Bounded Hot Cache
	cfg := manticore.Config{
		Peers:           []string{"127.0.0.1:9306"},
		Timeout:         5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		RetentionPeriod: 7 * 24 * time.Hour, // Keep only 7 days in Manticore
		MaxRowsCapacity: 100000,            // Strict upper row limit
		UseColumnar:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := manticore.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer client.Close()

	fmt.Println("=== 1. Execution & Durable Snapshotting to Universal S3 ===")
	// When process executes in NativeBPM Core:
	instanceID := uint64(20001)
	rawMemorySnapshot := []byte("DURABLE_EXECUTION_MEMORY_SNAPSHOT_WASMEE_V1")
	s3.PutSnapshot(instanceID, rawMemorySnapshot)

	fmt.Println("\n=== 2. Projecting metadata to Manticore Bounded Cache ===")
	inst := &manticore.ProcessInstance{
		ID:         instanceID,
		ProcessKey: "loan_origination",
		Version:    1,
		Status:     "COMPLETED",
		TenantID:   100,
		Assignee:   "officer_bob",
		StartTime:  time.Now().Add(-10 * 24 * time.Hour), // 10 days ago (older than 7 days)
		EndTime:    time.Now().Add(-10*24*time.Hour + 30*time.Minute),
		DurationMs: 1800000,
		Amount:     50000.00,
	}

	_ = client.UpsertInstance(ctx, inst)
	fmt.Println("  [Manticore] Metadata projected for fast search")

	fmt.Println("\n=== 3. Enforcing Bounded Retention Policy (Rolling Window) ===")
	// Background Janitor evicts completed records older than RetentionPeriod (7 days)
	cutoff := time.Now().Add(-cfg.RetentionPeriod)
	fmt.Printf("  Running PruneExpired for records completed before %s...\n", cutoff.Format(time.RFC3339))
	
	if err := client.PruneExpired(ctx, cutoff); err != nil {
		log.Printf("  Prune notice (in mock/offline mode): %v", err)
	} else {
		fmt.Println("  ✓ Expired records successfully evicted from Manticore")
	}

	fmt.Println("\n=== 4. Enforcing Hard Capacity Limits (FIFO Eviction) ===")
	if err := client.EnforceMaxCapacity(ctx, cfg.MaxRowsCapacity); err != nil {
		log.Printf("  Capacity check notice: %v", err)
	} else {
		fmt.Printf("  ✓ Capacity enforced: table process_instances <= %d rows\n", cfg.MaxRowsCapacity)
	}

	fmt.Println("\n=== 5. Reclaiming Disk Space via Compaction ===")
	if err := client.Optimize(ctx, "process_instances"); err != nil {
		log.Printf("  Optimize notice: %v", err)
	} else {
		fmt.Println("  ✓ Triggered background chunk compaction (OPTIMIZE TABLE)")
	}

	fmt.Println("\nResult:")
	fmt.Println("- Universal S3 holds 100% of historical memory snapshots and audit journals.")
	fmt.Println("- Manticore holds strictly bounded hot working set, consuming minimal SSD/RAM.")
}
