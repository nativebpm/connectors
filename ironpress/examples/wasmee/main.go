package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nativebpm/connectors/ironpress"
	"github.com/nativebpm/connectors/wasmee"
)

// Simple in-memory SnapshotStore implementation for local demo testing
type inMemoryStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]wasmee.OplogEntry
	metadata  map[string]*wasmee.InstanceMeta
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]wasmee.OplogEntry),
		metadata:  make(map[string]*wasmee.InstanceMeta),
	}
}

func (s *inMemoryStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.snapshots[id] = snapshot
	return nil
}

func (s *inMemoryStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	data, ok := s.snapshots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (s *inMemoryStore) DeleteSnapshot(ctx context.Context, id string) error {
	delete(s.snapshots, id)
	return nil
}

func (s *inMemoryStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *inMemoryStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	return s.deltas[id], nil
}

func (s *inMemoryStore) TruncateDeltas(ctx context.Context, id string) error {
	delete(s.deltas, id)
	return nil
}

func (s *inMemoryStore) SaveOplog(ctx context.Context, id string, entry wasmee.OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *inMemoryStore) LoadOplog(ctx context.Context, id string) ([]wasmee.OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *inMemoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []wasmee.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *inMemoryStore) SaveMetadata(ctx context.Context, meta *wasmee.InstanceMeta) (bool, error) {
	s.metadata[meta.InstanceID] = meta
	return true, nil
}

func (s *inMemoryStore) LoadMetadata(ctx context.Context, id string) (*wasmee.InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return meta, nil
}

func main() {
	wasmPath := flag.String("wasm", "/tmp/ironpress_wasm/bin/ironpress.wasm", "Path to ironpress.wasm file")
	wasmeeAddr := flag.String("wasmee", "http://localhost:8081", "Address of the WASMEE execution daemon")
	outFile := flag.String("out", "output_wasmee.pdf", "Path to save the generated PDF")
	flag.Parse()

	// 1. Check if WASM file exists
	if _, err := os.Stat(*wasmPath); os.IsNotExist(err) {
		log.Fatalf("WASM module not found at %s. Please compile it first.", *wasmPath)
	}

	// 2. Read Wasm bytes
	wasmBytes, err := os.ReadFile(*wasmPath)
	if err != nil {
		log.Fatalf("Failed to read WASM file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 3. Initialize mock snapshot store and client
	store := newInMemoryStore()
	meta := &wasmee.InstanceMeta{
		InstanceID: "session-ironpress-123",
		WasmHash:   "ironpress_wasm_hash",
		Version:    0,
	}
	_, _ = store.SaveMetadata(ctx, meta)

	log.Printf("Connecting to WASMEE daemon at %s...", *wasmeeAddr)
	client := ironpress.NewClient(ironpress.WithWasmee(*wasmeeAddr, store, wasmBytes))

	htmlContent := "<html><body><h1>Hello from WASMEE Durable Execution</h1></body></html>"

	log.Println("Executing ironpress PDF conversion via WASMEE execution runner...")
	pdfBytes, err := client.Convert(ironpress.WASMEE_Mode).
		SessionID("session-ironpress-123").
		HTML(htmlContent).
		Do(ctx)

	if err != nil {
		// Log helper in case of connection failure (server not started)
		log.Printf("[NOTE] WASMEE test requires the wasmee server running on %s. Error: %v", *wasmeeAddr, err)
		return
	}

	// Save output
	if err := os.WriteFile(*outFile, pdfBytes, 0644); err != nil {
		log.Fatalf("Failed to save output PDF: %v", err)
	}

	fmt.Printf("Successfully generated PDF via WASMEE: %s (%d bytes)\n", *outFile, len(pdfBytes))
}
