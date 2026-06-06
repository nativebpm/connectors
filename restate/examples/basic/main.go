package main

import (
	"context"
	"fmt"
	"log"

	restateconn "github.com/nativebpm/connectors/restate"
	restate "github.com/restatedev/sdk-go"
)

// Counter is a stateful Virtual Object in Restate.
type Counter struct{}

// Add increments the counter by the specified amount and returns the new value.
func (Counter) Add(ctx restate.ObjectContext, amount int) (int, error) {
	// Retrieve count from Restate K/V store.
	val, err := restate.Get[int](ctx, "count")
	if err != nil {
		return 0, fmt.Errorf("failed to get count: %w", err)
	}

	newVal := val + amount

	// Update state in Restate store.
	restate.Set(ctx, "count", newVal)

	// Execute side effect durably.
	_, err = restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		fmt.Printf("Durable side effect logged: counter increased by %d, new total = %d\n", amount, newVal)
		return "ok", nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to run side effect: %w", err)
	}

	return newVal, nil
}

// Get queries the current counter value using read-only shared context.
func (Counter) Get(ctx restate.ObjectSharedContext) (int, error) {
	val, err := restate.Get[int](ctx, "count")
	if err != nil {
		return 0, err
	}
	return val, nil
}

func main() {
	cfg, err := restateconn.NewConfigBuilder().
		FromEnv().
		WithHostPort("0.0.0.0:9080").
		Build()
	if err != nil {
		log.Fatalf("Failed to build config: %v", err)
	}

	fmt.Printf("Starting Counter Virtual Object service on %s...\n", cfg.HostPort)
	srv := restateconn.NewServer(cfg)
	srv.Bind(Counter{})

	if err := srv.Start(context.Background()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
