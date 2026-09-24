# NativeBPM Manticore Connector (`connectors/manticore`)

High-performance, pure Zero-SDK connector for Manticore Search, tailored for CQRS projections, Tasklist indexing, and full-text process monitoring in NativeBPM.

## Key Features

1. **Pure Zero-SDK**:
   - Zero vendor SDK dependencies.
   - Built strictly on top of Go's `database/sql` using the standard MySQL Wire Protocol (`:9306`).
   - Fully `CGO_ENABLED=0` compatible.

2. **Bounded Cache & Universal S3 Persistence**:
   - Manticore holds a **strictly bounded working set** of active instances and recent history (configurable TTL via `RetentionPeriod` and row limits via `MaxRowsCapacity`).
   - Universal S3 (AWS S3, Cloudflare R2, or local Garage S3 over UDS) remains the authoritative **Source of Truth** for durable execution snapshots.
   - Automatic pruning (`PruneExpired`, `EnforceMaxCapacity`) and background chunk compaction (`Optimize`).

3. **Manticore Peers & P2P Broadcast**:
   - Multi-peer active-active connection pool (`Peers: ["127.0.0.1:9306", "10.0.0.2:9306"]`).
   - Concurrent broadcast writes (`Broadcast`) for `REPLACE INTO`, `INSERT`, and `UPDATE`.
   - Balanced round-robin read query distribution across healthy peers.
   - Fail-fast error propagation (Strict Zero Fallback invariant).

4. **Columnar Storage & Sub-Millisecond Queries**:
   - Real-time RT tables with Manticore Columnar Storage for scalar attributes (`engine='columnar'`).
   - In-place scalar updates (`UPDATE ... SET status = ?`).
   - Full-text search over error messages, payloads, and variables via `MATCH(...)`.

## Installation & Usage

```go
cfg := manticore.Config{
    Peers:           []string{"127.0.0.1:9306", "127.0.0.1:9316"},
    Timeout:         3 * time.Second,
    MaxOpenConns:    20,
    MaxIdleConns:    10,
    RetentionPeriod: 30 * 24 * time.Hour,
    MaxRowsCapacity: 500000,
    UseColumnar:     true,
}

client, err := manticore.NewClient(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Initialize RT schema
_ = client.InitSchema(ctx)

// Project instance
_ = client.UpsertInstance(ctx, &manticore.ProcessInstance{
    ID:         10042,
    ProcessKey: "order_approval",
    Status:     "RUNNING",
    StartTime:  time.Now(),
    Amount:     1250.00,
})

// Fast in-place status update (< 100 µs)
_ = client.UpdateStatus(ctx, 10042, "COMPLETED", 1250)

// Fast multi-attribute and full-text search
res, _ := client.SearchInstances(ctx, manticore.SearchFilter{
    Status: "COMPLETED",
    Limit:  20,
})
```

## Running Tests & Benchmarks

```bash
# Run unit & mock tests
CGO_ENABLED=0 go test -v ./...

# Run load test benchmarks
CGO_ENABLED=0 go test -bench=. -benchmem ./...
```
