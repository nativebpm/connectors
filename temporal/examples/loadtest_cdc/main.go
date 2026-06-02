package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"go.temporal.io/sdk/client"
)

var (
	startedInstances   atomic.Int64
	completedInstances atomic.Int64
	failedInstances    atomic.Int64

	startTimes  sync.Map
	durationsMu sync.Mutex
	durations   []time.Duration
)

// Local Workflow was removed, using shared temporal.GreetCDCWorkflow

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	concurrency := 50
	if val := os.Getenv("LOAD_CONCURRENCY"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			concurrency = parsed
		}
	}

	totalProcesses := 1000
	if val := os.Getenv("LOAD_PROCESSES_COUNT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			totalProcesses = parsed
		}
	}

	submissionDelayMs := 0
	if val := os.Getenv("LOAD_SUBMISSION_DELAY_MS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			submissionDelayMs = parsed
		}
	}

	logger.Info("TEMPORAL CDC LOAD TEST INITIALIZED",
		"concurrency", concurrency,
		"total_processes", totalProcesses,
		"submission_delay_ms", submissionDelayMs,
	)

	cfg := temporal.LoadFromEnv()

	// Initialize Temporal client
	c, err := temporal.NewClient(cfg)
	if err != nil {
		logger.Error("Failed to create Temporal client", "error", err)
		return
	}
	defer c.Close()

	// Initialize database connections for delegation activities
	cdcActs, err := temporal.NewCDCActivities(cfg)
	if err != nil {
		logger.Error("Failed to connect to database for CDC activities", "error", err)
		return
	}
	defer cdcActs.Close()

	// Initialize standard Temporal worker (for orchestration and delegation activities)
	w := temporal.NewWorker(c, cfg.TaskQueue)
	w.RegisterWorkflow(temporal.GreetCDCWorkflow)
	w.RegisterActivity(cdcActs)

	err = w.Start()
	if err != nil {
		logger.Error("Failed to start Temporal worker", "error", err)
		return
	}
	defer w.Stop()

	// Initialize high-performance Sequin CDC Worker
	sequinURL := os.Getenv("SEQUIN_URL")
	if sequinURL == "" {
		sequinURL = "http://127.0.0.1:7386"
	}
	sequinConsumer := os.Getenv("SEQUIN_CONSUMER")
	if sequinConsumer == "" {
		sequinConsumer = "temporal_tasks"
	}

	cdcWorker, err := temporal.NewSequinCDCWorker(c, sequinURL, sequinConsumer, logger)
	if err != nil {
		logger.Error("Failed to create Sequin CDC worker", "error", err)
		return
	}
	cdcWorker.SetMaxConcurrency(concurrency)

	// Register CDC handler
	cdcWorker.RegisterHandler("greet-cdc", func(ctx context.Context, payload string) (string, error) {
		// Quickly execute the task bypassing Temporal Matcher
		return fmt.Sprintf("Hello, %s (processed via WAL CDC)!", payload), nil
	})

	// Start CDC worker
	cdcWorker.Start(context.Background())

	doneChan := make(chan struct{})

	// Monitor progress
	startTime := time.Now()
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-doneChan:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime).Seconds()
				completed := completedInstances.Load()
				started := startedInstances.Load()
				failed := failedInstances.Load()
				logger.Info("Progress",
					"elapsed_seconds", fmt.Sprintf("%.1f", elapsed),
					"started", started,
					"completed", completed,
					"failed", failed,
					"percentage", fmt.Sprintf("%.1f%%", float64(completed)/float64(totalProcesses)*100),
				)
			}
		}
	}()

	// Start instances concurrently in goroutines
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i := 1; i <= totalProcesses; i++ {
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			workflowID := "loadtest-cdc-workflow-" + uuid.New().String()
			options := client.StartWorkflowOptions{
				ID:        workflowID,
				TaskQueue: cfg.TaskQueue,
			}

			startTimes.Store(workflowID, time.Now())

			// Start workflow
			run, err := c.ExecuteWorkflow(context.Background(), options, temporal.GreetCDCWorkflow, fmt.Sprintf("Load-%d", num))
			if err != nil {
				failedInstances.Add(1)
				logger.Error("Error starting CDC workflow", "id", workflowID, "error", err)
				return
			}
			startedInstances.Add(1)

			// Wait for completion
			var result string
			err = run.Get(context.Background(), &result)
			if err != nil {
				failedInstances.Add(1)
				logger.Error("Error executing CDC workflow", "id", workflowID, "error", err)
				return
			}

			// Compute latency
			if startVal, ok := startTimes.Load(workflowID); ok {
				if st, ok := startVal.(time.Time); ok {
					durationsMu.Lock()
					durations = append(durations, time.Since(st))
					durationsMu.Unlock()
					startTimes.Delete(workflowID)
				}
			}

			completedInstances.Add(1)

			if submissionDelayMs > 0 {
				time.Sleep(time.Duration(submissionDelayMs+rand.Intn(10)) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(doneChan)

	totalDuration := time.Since(startTime)

	// Compute percentiles
	durationsMu.Lock()
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	p50 := time.Duration(0)
	p90 := time.Duration(0)
	p95 := time.Duration(0)
	p99 := time.Duration(0)
	avg := time.Duration(0)

	if len(durations) > 0 {
		var total time.Duration
		for _, d := range durations {
			total += d
		}
		avg = total / time.Duration(len(durations))

		getPercentile := func(pct float64) time.Duration {
			idx := int(float64(len(durations)) * pct / 100.0)
			if idx >= len(durations) {
				idx = len(durations) - 1
			}
			return durations[idx]
		}
		p50 = getPercentile(50)
		p90 = getPercentile(90)
		p95 = getPercentile(95)
		p99 = getPercentile(99)
	}
	durationsMu.Unlock()

	// Output results
	logger.Info("TEMPORAL CDC LOAD TEST RESULTS",
		"total_duration", totalDuration,
		"submitted", totalProcesses,
		"completed", completedInstances.Load(),
		"failed", failedInstances.Load(),
		"throughput_rps", fmt.Sprintf("%.2f", float64(completedInstances.Load())/totalDuration.Seconds()),
		// 1 Workflow + 1 Activity + 1 Signal = 3 tasks per instance in the engine
		"task_throughput_tps", fmt.Sprintf("%.2f", float64(completedInstances.Load()*3)/totalDuration.Seconds()),
		"p50_latency_ms", p50.Milliseconds(),
		"p90_latency_ms", p90.Milliseconds(),
		"p95_latency_ms", p95.Milliseconds(),
		"p99_latency_ms", p99.Milliseconds(),
		"avg_latency_ms", avg.Milliseconds(),
	)
}
