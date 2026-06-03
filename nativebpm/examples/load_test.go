package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nativebpm/connectors/nativebpm"
)

func TestLoad_Concurrently(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Deploy the process first
	_, err = client.Deploy("userTaskProcess", "User Task Process").
		XML([]byte(userTaskBPMN)).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	concurrency := 20
	if env := os.Getenv("LOAD_CONCURRENCY"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			concurrency = val
		}
	}

	iterations := 5
	if env := os.Getenv("LOAD_ITERATIONS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			iterations = val
		}
	}

	var wg sync.WaitGroup
	start := time.Now()

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				instanceID := fmt.Sprintf("load-inst-%d-%d-%d", workerID, i, time.Now().UnixNano())

				// 1. Start instance
				pi, err := client.StartProcessInstance("userTaskProcess").
					InstanceID(instanceID).
					Variable("applicant", "Worker").
					Variable("worker_id", workerID).
					Send(ctx)
				if err != nil {
					t.Errorf("StartProcessInstance failed for %s: %v", instanceID, err)
					continue
				}

				// 2. Complete task
				if len(pi.WaitingTokens) > 0 {
					taskID := pi.WaitingTokens[0]
					_, err = client.CompleteTask(pi.ID, taskID).
						Variable("approved", true).
						Send(ctx)
					if err != nil {
						t.Errorf("CompleteTask failed for %s task %s: %v", instanceID, taskID, err)
					}
				}
			}
		}(worker)
	}

	wg.Wait()
	duration := time.Since(start)
	totalRuns := concurrency * iterations
	t.Logf("Completed %d process executions in %v (RPS: %.2f)", totalRuns, duration, float64(totalRuns)/duration.Seconds())
}
