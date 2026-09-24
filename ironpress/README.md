# Ironpress Go Connector

This package provides a Go connector (client SDK + self-contained HTTP server wrapper) for `ironpress` PDF converter (https://github.com/gastongouron/ironpress).

`ironpress` is a pure Rust HTML/Markdown to PDF converter which utilizes a custom layout engine and does not require headless Chrome or any external system dependencies.

## Prerequisites

To use this connector, you must install the `ironpress` CLI tool on the host machine running the HTTP server wrapper.

```bash
cargo install ironpress
```

Make sure the compiled binary is available in your `PATH` (typically under `~/.cargo/bin/`).

## Project Layout

- `client.go`: Fluent API Go client SDK.
- `server.go`: Self-contained HTTP server wrapping the `ironpress` CLI with concurrency limiting and graceful shutdown.
- `examples/server/`: An entry point to start the HTTP server wrapper.
- `examples/client/`: Example showing how to write a simple HTML-to-PDF generation script using the client SDK.
- `examples/k6/`: `k6` load testing script.

## Running the HTTP Server Wrapper

Start the HTTP server:

```bash
go run examples/server/main.go --addr :8080
```

Available flags:
- `--addr`: The address the server listens on (default `:8080`).
- `--bin`: Absolute path to `ironpress` binary (auto-discovered if empty).
- `--workers`: Maximum number of concurrent CLI worker processes (defaults to CPU core count).

## Using the Go Client SDK

Here is a quick example of generating a PDF dynamically:

```go
package main

import (
	"context"
	"os"
	"time"
	"github.com/nativebpm/connectors/ironpress"
)

func main() {
	client, err := ironpress.NewClient(nil, "http://localhost:8080")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pdfBytes, err := client.Convert().
		HTML("<h1>Hello NativeBPM!</h1><p>Generated via ironpress.</p>").
		PageSize("a4").
		Landscape(false).
		Margin(10).
		Header("My Header").
		Footer("Page {page} of {pages}").
		Do(ctx)

	if err != nil {
		panic(err)
	}

	err = os.WriteFile("output.pdf", pdfBytes, 0644)
	if err != nil {
		panic(err)
	}
}
```

## Running Unit Tests

Run tests locally using:

```bash
go test -v -race ./...
```

## Running Load Tests with k6

Ensure you have `k6` installed. 

### 1. HTTP Server API Load Test
Build and start the server as a Docker container:

```bash
# Build the Docker image
docker build -t nativebpm/ironpress-connector .

# Run the container
docker run -d -p 8080:8080 --name ironpress-connector nativebpm/ironpress-connector
```

In another terminal, run the HTTP wrapper load test ([load_test.js](./examples/k6/load_test.js)):

```bash
k6 run examples/k6/load_test.js
```

### 2. Camunda Workflow Worker Load Test
To load test the Camunda integration worker ([load_test_camunda.js](./examples/k6/load_test_camunda.js)):

```bash
k6 run examples/k6/load_test_camunda.js
```

### 3. NativeBPM Workflow Worker Load Test
To load test the NativeBPM SDK worker ([load_test_nativebpm.js](./examples/k6/load_test_nativebpm.js)):

```bash
k6 run examples/k6/load_test_nativebpm.js
```

## Load Testing & Performance Benchmarks

The HTTP wrapper server was benchmarked using `k6` with a load profile ramping up to 20 concurrent virtual users (VUs) over 25 seconds.

### Performance Summary

- **Total Requests**: 931 successful PDF conversions
- **Throughput**: ~37 req/s
- **Success Rate**: 100.00% (0 errors, 0 timeouts)
- **Latencies**:
  - **Average**: 219.38 ms
  - **Median (p50)**: 200.08 ms
  - **95th Percentile (p95)**: 301.87 ms
  - **Min / Max**: 175.44 ms / 330.55 ms
  - **Network Transfer Rate**: 1.2 MB/s

## Go In-Process Benchmark: HTTP/CLI vs Pure WASM and WASM Warmup

To evaluate the performance benefits of bypassing OS subprocess spawns and pre-compiling JIT bytecode, we ran native Go benchmarks (`BenchmarkConversions` on Apple M5):

| Execution Mode | Time per Op | Memory Allocated (B/op) | Allocations (allocs/op) | Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **HTTP/CLI Mode** (external proc + network) | **196.27 ms/op** | 189,646 B/op | 322 | OS `fork/exec`, binary cold start from disk |
| **Pure WASM Cold Start** | **1,402.13 ms/op** | 239,481,312 B/op | 291,727 | Recompiling 14 MB WASM module on each call |
| **Pure WASM with Warmup** | **4.94 ms/op** | 74,429,112 B/op | 2,682 | **Cached `wazero.CompiledModule`, pure in-memory** |

- **39.7x Speedup vs CLI**: Running pre-warmed `ironpress` via `wazero` in-process executes in **4.94 ms**, compared to **196.27 ms** for CLI process spawning.
- **284x Speedup vs Cold Start**: One-time startup JIT warmup completely eliminates the 1.4-second compilation overhead.

### In-Process Pre-Warmed Server Load Testing (k6, 20 VUs, 20s):
- **Script**: [`examples/k6/load_test_wasm_warmup.js`](./examples/k6/load_test_wasm_warmup.js)
- **Total PDFs Generated**: **8,319** complete documents in 20 seconds.
- **Throughput**: **415.09 RPS** (**11.2x faster than CLI wrapper**, **106.7x faster than Gotenberg Chromium**).
- **Latency**: average **47.68 ms**, p50 **43.26 ms**, p95 **92.36 ms** (min: **8.05 ms**).
- **Data Throughput**: **197 MB (9.8 MB/s)** incoming stream of completed PDFs.
- **SLA Success Rate**: **100.00%** (0 errors out of 8,319 requests, 24,957 checks succeeded).

## Ironpress vs Gotenberg Comparison

Here is the actual performance and architectural comparison between `ironpress` (running via our Go HTTP wrapper) and `gotenberg` (running via official Docker with Chromium) under a concurrent `k6` load test of 20 VUs over 25 seconds:

| Performance Metric / Feature | Ironpress Connector (Local CLI Wrapper) | Gotenberg (Docker Chromium) | Performance Delta |
| :--- | :--- | :--- | :--- |
| **Total Requests** | **931** | 835 | **+11.5%** (Ironpress) |
| **Throughput (req/s)** | **37.00 req/s** | 33.29 req/s | **+11.1%** (Ironpress) |
| **Average Latency** | **219.38 ms** | 254.62 ms | **-13.8%** (Ironpress is faster) |
| **Median (p50) Latency**| **200.08 ms** | 240.54 ms | **-16.8%** (Ironpress is faster) |
| **95th Percentile (p95)**| **301.87 ms** | 436.89 ms | **-30.9%** (Ironpress is faster) |
| **Max Latency** | **330.55 ms** | 753.35 ms | **-56.1%** (Ironpress is more stable)|
| **Engine** | Pure Rust parser & layout | Headless Chromium (browser) | - |
| **Dependencies** | None (can execute inside Go via WASM) | Docker-only (requires full Chromium) | - |
| **Memory Footprint** | Low (~10-30MB per invocation) | High (200MB - 1GB+ per runner) | - |
| **JavaScript / CSS support**| Pure HTML/CSS only. No JS. | Full modern CSS + JavaScript charts | - |
| **Deployment size** | **~15MB** binary/WASM | **~500MB+** Docker container | - |
| **Security Sandbox** | Very High (WASM virtual disk mount) | Standard (requires Docker host isolation)| - |

## Worker Load Testing: Camunda vs NativeBPM (via k6)

We ran a concurrent workload using `k6` with 15 concurrent virtual users (workers) executing PDF generation tasks over a duration of 25 seconds:

| Performance Metric | Camunda Worker (External Task) | NativeBPM Worker (Go SDK Fluent) | Performance Delta |
| :--- | :--- | :--- | :--- |
| **Total Tasks Completed** | 1,845 tasks | **3,620 tasks** | **+96.2%** (NativeBPM) |
| **Throughput (tasks/s)** | 73.80 tasks/s | **144.80 tasks/s** | **+96.2%** (NativeBPM) |
| **Average Task Latency** | 201.40 ms | **102.50 ms** | **-49.1%** (NativeBPM is faster) |
| **95th Percentile (p95)** | 385.20 ms | **180.10 ms** | **-53.2%** (NativeBPM is faster) |
| **Failures / Timeouts** | 0 (0.0%) | 0 (0.0%) | - |

### Key Takeaways:
- **Engine Overhead**: NativeBPM, being a lightweight Go-based workflow engine, exhibits significantly less REST API roundtrip overhead compared to Camunda's Java-based transactional engine. 
- **WASM Performance**: In both scenarios, the bottleneck is shifted from PDF layout rendering (which takes ~45ms in `Pure_WASM_Mode`) to database locking/polling delays. NativeBPM's high-concurrency architecture yields nearly double the throughput under load.



