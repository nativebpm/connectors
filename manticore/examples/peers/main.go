package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nativebpm/connectors/manticore"
)

func main() {
	// Configure connection to multiple Manticore peers
	cfg := manticore.Config{
		Peers: []string{
			"127.0.0.1:9306", // Peer 1
			"127.0.0.1:9316", // Peer 2
			"127.0.0.1:9326", // Peer 3
		},
		Timeout:         3 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		RetentionPeriod: 14 * 24 * time.Hour,
		MaxRowsCapacity: 250000,
		UseColumnar:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := manticore.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize multi-peer client: %v", err)
	}
	defer client.Close()

	fmt.Printf("Configured %d peers: %v\n", len(client.Peers()), client.Peers())

	// Broadcast an incident event across all peers
	incidentID := uint64(time.Now().UnixNano())
	incident := &manticore.ProcessInstance{
		ID:           incidentID,
		ProcessKey:   "payment_settlement",
		Version:      3,
		Status:       "INCIDENT",
		TenantID:     7,
		Assignee:     "support_tier2",
		StartTime:    time.Now().Add(-5 * time.Minute),
		Amount:       25000.00,
		ErrorMessage: "ACTIVITY_EXECUTION_TIMEOUT: wasmee worker billing-gateway unresponsive after 5000ms",
	}

	fmt.Println("Broadcasting incident to all peers concurrently...")
	if err := client.UpsertInstance(ctx, incident); err != nil {
		log.Printf("Notice: in offline dev without 3 live peers, error expected: %v", err)
	} else {
		fmt.Printf("✓ Incident %d successfully synchronized across all peers\n", incidentID)
	}

	// Reading uses round-robin load balancing across healthy peers
	fmt.Println("Executing balanced read query across peers...")
	filter := manticore.SearchFilter{
		Status:    "INCIDENT",
		QueryText: "ACTIVITY_EXECUTION_TIMEOUT",
		Limit:     5,
	}
	res, err := client.SearchInstances(ctx, filter)
	if err != nil {
		log.Printf("Query notice: %v", err)
	} else {
		fmt.Printf("✓ Found %d active incidents across peers\n", res.Total)
	}
}
