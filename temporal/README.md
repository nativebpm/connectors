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

## 3. Load Testing & CDC Delegation

The module supports two task execution architectures:
1. **Classic gRPC Polling (No CDC)**: Standard Temporal task queue polling mechanism where workers poll task queues directly via gRPC.
2. **WAL CDC Delegation (via Sequin)**: High-performance delegation scheme to offload Temporal. An activity writes a task to `custom_task_queue` in PostgreSQL, Sequin reads PostgreSQL WAL logs and streams them to the CDC Pull queue, where lightweight HTTP workers pull jobs and signal completions back to Temporal. DB schemas and publications are fully managed by **Atlas** migration tool.

### Running the Load Tests

1. Make sure the Docker infrastructure is running (`make infra-up`).
2. To run the classic long polling test (No CDC):
   ```bash
   make run-loadtest
   ```
3. To run the CDC delegation load test:
   ```bash
   make run-loadtest-cdc
   ```

---

## 4. Performance Metrics

Under benchmark testing, we evaluated both options under concurrent scaling pressures:

* **Classic gRPC Polling (No CDC)**:
  - **1000 processes** (2,000 tasks): Completed in **11.58s** at **86.32 RPS / 172.64 TPS** (Latency: p50=375ms, p90=1002ms, p99=1522ms, average=487ms. 100% success rate).
* **WAL CDC Delegation via Sequin (Optimized)**:
  - **3000 processes** (9,000 task transitions: 3,000 starts, 3,000 WAL writes, 3,000 signals): Completed in **58.52s** at **51.26 RPS / 153.78 TPS** (Latency: p50=3.3s, p90=3.6s, p95=3.9s, p99=6.2s, average=2.9s. 100% success rate with zero database bottlenecks).
  - *Note*: Optimizing the Go TCP connection pool (`SetMaxOpenConns(150)`), disabling synchronous commit/fsync in PostgreSQL for testing, and scaling Temporal history shards to `512` yielded a 2x throughput speedup and cut p99 latency by ~4x (from 22.7s down to 6.2s).

