# Temporal.io Go Connector

A high-performance Go wrapper for Temporal.io Go SDK, providing simplified client/worker APIs, structured environment-based configurations, and preconfigured Docker architectures.

---

## 1. Directory Structure

```
temporal/
├── config.go         # Connection & TLS configuration
├── client.go         # Wrapper client for workflow starts, queries, and signals
├── worker.go         # Wrapper worker for registering workflows and activities
├── docker/           # Preconfigured Docker Compose infrastructure with PostgreSQL
└── examples/
    ├── helloworld/   # Basic Workflow & Activity execution
    ├── signal/       # Using Signals and Queries to handle interactive state
    ├── saga/         # Saga pattern implementation with LIFO compensations
    └── loadtest/     # Concurrent high-throughput load testing tool
```

---

## 2. Infrastructure Setup (Docker Compose)

The module includes a preconfigured Docker Compose file running **Temporal Server v1.24.2** backed by **PostgreSQL 14**.

To start the infrastructure:
```bash
# Using Makefile from temporal/ directory:
make infra-up

# Or manually:
cd temporal/docker
docker compose up -d
```
Once started:
- **Temporal Web UI** is available at: [http://localhost:8233](http://localhost:8233)
- **Temporal gRPC Endpoint**: `localhost:7233`

---

## 3. High-Throughput Load Testing

The package includes a load testing utility in `examples/loadtest` designed to evaluate Temporal engine performance under concurrent pressures.

### Running the Load Test
1. Make sure Temporal Docker infrastructure is running.
2. Run the load test tool:
   ```bash
   # Using Makefile:
   make run-loadtest

   # Or manually:
   LOAD_CONCURRENCY=50 LOAD_PROCESSES_COUNT=1000 go run temporal/examples/loadtest/main.go
   ```

3. The tool logs progress and output detailed summary metrics, including **p50, p90, p95, p99 latencies**, **RPS**, and **TPS (Task Throughput)**.
