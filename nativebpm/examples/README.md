# NativeBPM Connector Examples

This directory contains executable examples demonstrating how to interact with the NativeBPM platform REST API using the Go SDK.

## Structure

*   **[`simple/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/simple)**: A basic workflow containing a single manual `User Task` (Approval). Demonstrates process deployment, instance startup, task completion, and audit log retrieval.
*   **[`complex/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/complex)**: A process with exclusive gateway branching (`XOR`). Runs two scenarios concurrently to demonstrate how execution routing behaves depending on input variables (`amount`).
*   **[`loadtest/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/loadtest)**: High-concurrency performance benchmark. Runs multiple parallel execution workers to measure NativeBPM platform throughput and RPS limits.
*   **[`hybrid-camunda/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/hybrid-camunda)**: A hybrid architecture where a Camunda engine delegates tasks to NativeBPM. The Go worker polls Camunda, invokes the NativeBPM REST API, and returns the result.

## Prerequisites

1.  Start the NativeBPM platform mock containers (PostgreSQL, MinIO, and Server):
    ```bash
    docker compose up -d
    ```
    This launches the server at `http://localhost:8080`.

2.  Ensure Go dependencies are synchronized:
    ```bash
    go mod tidy
    ```

## Running the Examples

### 1. Simple Example
Navigate to the `simple` directory and run:
```bash
cd simple
go run main.go
```

### 2. Complex Example
Navigate to the `complex` directory and run:
```bash
cd complex
go run main.go
```

### 3. Load Testing
Navigate to the `loadtest` directory and run:
```bash
cd loadtest
# Run standard load test (concurrency 20, iterations 5)
go test -v .

# Run with custom load settings
LOAD_CONCURRENCY=50 LOAD_ITERATIONS=10 go test -v .
```

#### Load Test Benchmarks (Executed on 2026-06-04)
| Concurrency (Workers) | Iterations | Total Runs | RPS | Status | Notes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 50 | 10 | 500 | **511.24** | **PASS** | Stable execution, 0 database bottlenecks. |
| 70 | 10 | 700 | **658.49** | **PASS** | High throughput, 0 database bottlenecks. |
| 100 | 10 | 1000 | **759.04** | **PASS** | Passes successfully after optimizing PostgreSQL configuration (`max_connections=500`). |
| 150 | 10 | 1500 | **756.80** | **PASS** | Peak stable throughput under extreme concurrency. |

### 4. Hybrid Camunda Bridge
Ensure you have Camunda running (e.g. on port `8082`), then navigate to the `hybrid-camunda` directory and run:
```bash
cd hybrid-camunda
go run main.go
```
