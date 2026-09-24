package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nativebpm/connectors/manticore"
)

func main() {
	cfg := manticore.Config{
		Peers:           []string{"127.0.0.1:9306"},
		Timeout:         5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		RetentionPeriod: 30 * 24 * time.Hour,
		MaxRowsCapacity: 500000,
		UseColumnar:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := manticore.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	defer client.Close()

	fmt.Println("1. Initializing Manticore RT tables with columnar attributes...")
	if err := client.InitSchema(ctx); err != nil {
		log.Printf("InitSchema notice (tables might exist): %v", err)
	}

	fmt.Println("2. Inserting running process instance projection (REPLACE INTO)...")
	instanceID := uint64(time.Now().UnixNano())
	inst := &manticore.ProcessInstance{
		ID:            instanceID,
		ProcessKey:    "order_fulfillment_v2",
		Version:       1,
		Status:        "RUNNING",
		TenantID:      42,
		Assignee:      "operator_alice",
		StartTime:     time.Now(),
		Amount:        9450.00,
		ErrorMessage:  "",
		VariablesJSON: `{"currency":"USD","customer":"Acme Corp","items":12}`,
	}

	if err := client.UpsertInstance(ctx, inst); err != nil {
		log.Fatalf("Failed to upsert instance: %v", err)
	}
	fmt.Printf("✓ Process instance %d successfully projected to Manticore\n", instanceID)

	fmt.Println("3. Performing in-place scalar update (transition to COMPLETED)...")
	if err := client.UpdateStatus(ctx, instanceID, "COMPLETED", 3450); err != nil {
		log.Fatalf("Failed to update status: %v", err)
	}
	fmt.Printf("✓ Process instance %d status updated in-place\n", instanceID)

	fmt.Println("4. Searching instances with filters and pagination...")
	filter := manticore.SearchFilter{
		ProcessKey: "order_fulfillment_v2",
		Status:     "COMPLETED",
		Limit:      10,
	}
	res, err := client.SearchInstances(ctx, filter)
	if err != nil {
		log.Fatalf("Failed to search instances: %v", err)
	}
	fmt.Printf("✓ Found %d instances matching query\n", len(res.Instances))
	for _, found := range res.Instances {
		fmt.Printf("  - ID: %d, Key: %s, Status: %s, Amount: $%.2f\n",
			found.ID, found.ProcessKey, found.Status, found.Amount)
	}
}
