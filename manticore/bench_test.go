package manticore_test

import (
	"context"
	"testing"
	"time"

	"github.com/nativebpm/connectors/manticore"
	"github.com/stretchr/testify/require"
)

func BenchmarkUpsertInstance_SinglePeer(b *testing.B) {
	peer := startMockPeerBench(b)
	defer peer.Close()

	cfg := manticore.Config{
		Peers:           []string{peer.addr},
		Timeout:         5 * time.Second,
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		RetentionPeriod: 24 * time.Hour,
		MaxRowsCapacity: 500000,
	}

	ctx := context.Background()
	client, err := manticore.NewClient(ctx, cfg)
	require.NoError(b, err)
	defer client.Close()

	inst := &manticore.ProcessInstance{
		ID:         1,
		ProcessKey: "bench_process",
		Version:    1,
		Status:     "RUNNING",
		TenantID:   10,
		Assignee:   "worker_bench",
		StartTime:  time.Now(),
		Amount:     99.99,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		inst.ID = uint64(i + 1)
		if err := client.UpsertInstance(ctx, inst); err != nil {
			b.Fatalf("upsert failed: %v", err)
		}
	}
}

func BenchmarkUpsertInstance_DualPeers_Broadcast(b *testing.B) {
	peer1 := startMockPeerBench(b)
	defer peer1.Close()

	peer2 := startMockPeerBench(b)
	defer peer2.Close()

	cfg := manticore.Config{
		Peers:           []string{peer1.addr, peer2.addr},
		Timeout:         5 * time.Second,
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		RetentionPeriod: 24 * time.Hour,
		MaxRowsCapacity: 500000,
	}

	ctx := context.Background()
	client, err := manticore.NewClient(ctx, cfg)
	require.NoError(b, err)
	defer client.Close()

	inst := &manticore.ProcessInstance{
		ID:         1,
		ProcessKey: "bench_process",
		Version:    1,
		Status:     "RUNNING",
		TenantID:   10,
		Assignee:   "worker_bench",
		StartTime:  time.Now(),
		Amount:     99.99,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		inst.ID = uint64(i + 1)
		if err := client.UpsertInstance(ctx, inst); err != nil {
			b.Fatalf("broadcast upsert failed: %v", err)
		}
	}
}

func BenchmarkUpdateStatus_InPlace(b *testing.B) {
	peer := startMockPeerBench(b)
	defer peer.Close()

	cfg := manticore.Config{
		Peers:           []string{peer.addr},
		Timeout:         5 * time.Second,
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		RetentionPeriod: 24 * time.Hour,
		MaxRowsCapacity: 500000,
	}

	ctx := context.Background()
	client, err := manticore.NewClient(ctx, cfg)
	require.NoError(b, err)
	defer client.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := client.UpdateStatus(ctx, uint64(i+1), "COMPLETED", 120); err != nil {
			b.Fatalf("update status failed: %v", err)
		}
	}
}

func startMockPeerBench(b *testing.B) *mockMySQLPeer {
	l, err := startMockPeerListener()
	if err != nil {
		b.Fatalf("failed to listen: %v", err)
	}
	peer := &mockMySQLPeer{
		listener: l,
		addr:     l.Addr().String(),
	}
	go peer.serve()
	return peer
}
