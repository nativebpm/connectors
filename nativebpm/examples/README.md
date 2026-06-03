# NativeBPM Connector Examples

This directory contains executable examples demonstrating how to interact with the NativeBPM platform REST API using the Go SDK.

## Structure

*   **[`simple/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/simple)**: A basic workflow containing a single manual `User Task` (Approval). Demonstrates process deployment, instance startup, task completion, and audit log retrieval.
*   **[`complex/`](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/complex)**: A process with exclusive gateway branching (`XOR`). Runs two scenarios concurrently to demonstrate how execution routing behaves depending on input variables (`amount`).
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

### 3. Hybrid Camunda Bridge
Ensure you have Camunda running (e.g. on port `8082`), then navigate to the `hybrid-camunda` directory and run:
```bash
cd hybrid-camunda
go run main.go
```
