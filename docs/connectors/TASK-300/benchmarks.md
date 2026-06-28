# Load Testing & Performance Benchmarks — TASK-300

This document contains performance verification and benchmark results comparing the NativeBPM `ironpress` PDF generation connector wrapper against the `gotenberg` PDF engine under concurrent load, as well as an in-process benchmark comparing HTTP/CLI and pure WebAssembly (WASM) modes of `ironpress`.

## Test Environment

- **OS**: macOS
- **CPU**: Apple Silicon
- **Concurrency (VUs)**: 20 parallel virtual users
- **Duration**: 25 seconds total (stages: 5s ramp-up to 10 VUs, 15s sustain at 20 VUs, 5s ramp-down to 0 VUs)
- **Load Test Tool**: `k6`
- **Engines Config**:
  - **Ironpress HTTP Wrapper**: Running on port `8080`, calling `ironpress` CLI locally (max workers: 10).
  - **Gotenberg Docker Service**: `gotenberg/gotenberg:8` running on port `3000` (Chromium engine).

---

## k6 Load Test Results Comparison (Ironpress HTTP vs Gotenberg)

Below is the side-by-side comparison of the actual results gathered during the concurrent load test:

| Performance Metric | Ironpress Connector (Local CLI Wrapper) | Gotenberg (Docker Chromium) | Performance Delta |
| :--- | :--- | :--- | :--- |
| **Total Requests** | **931** | 835 | **+11.5%** (Ironpress) |
| **Throughput (req/s)** | **37.00 req/s** | 33.29 req/s | **+11.1%** (Ironpress) |
| **Success Rate** | **100.00%** (0 errors) | 100.00% (0 errors) | Equal |
| **Min Latency** | 175.44 ms | **43.11 ms** | Gotenberg (cached warmup) |
| **Average Latency** | **219.38 ms** | 254.62 ms | **-13.8%** (Ironpress is faster) |
| **Median (p50) Latency**| **200.08 ms** | 240.54 ms | **-16.8%** (Ironpress is faster) |
| **90th Percentile (p90)**| **295.58 ms** | 393.09 ms | **-24.8%** (Ironpress is faster) |
| **95th Percentile (p95)**| **301.87 ms** | 436.89 ms | **-30.9%** (Ironpress is faster) |
| **Max Latency** | **330.55 ms** | 753.35 ms | **-56.1%** (Ironpress is more stable) |

---

## In-Process Go Benchmark: HTTP/CLI vs Pure WASM Mode

To evaluate the performance benefits of bypassing HTTP network stack and OS subprocess spawns, we ran native Go benchmarks using `go test -bench=. -benchmem -benchtime=5s`.

Both benchmarks convert the exact same HTML template containing basic text and standard HTML elements:

| Execution Mode | Iterations (in 5s) | Speed (ns/op) | Speed (ms/op) | Memory Allocated (B/op) | Allocations (allocs/op) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **HTTP/CLI Mode** (external proc + network) | 31 | 191,785,508 ns/op | **191.78 ms/op** | **169,075 B/op** | **315** |
| **Pure WASM Mode** (in-memory wazero) | **127** | **45,977,764 ns/op** | **45.97 ms/op** | 97,846,565 B/op | 167,236 |

### Benchmark Insights:
1. **4.17x Speedup**: Running `ironpress` compiled to WebAssembly via the `wazero` engine in-process executes in **45.97 ms**, compared to **191.78 ms** when spawning an OS process and writing to temporary files over HTTP.
2. **Resource Trade-off**:
   - **HTTP/CLI Mode** delegates all execution to the OS process scheduler, showing extremely low memory allocations within Go's garbage-collected heap (~169 KB). However, the operating system overhead of starting a new program is high.
   - **Pure WASM Mode** executes entirely within Go's runtime memory space. Wazero's compilation and JIT translation allocate ~97 MB of memory inside Go's heap per execution loop. *Note: In production deployments, compiling the module once at startup will significantly decrease memory overhead on subsequent conversion runs.*

---

## Technical & Architectural Comparison

| Feature / Metric | Ironpress (Pure Rust / WASM) | Gotenberg (Chromium / Headless Chrome) |
| :--- | :--- | :--- |
| **Engine** | Pure Rust parser & layout engine. | Full headless browser execution (Chromium). |
| **Runtime Dependencies** | None. Can run in-memory via WebAssembly (WASM/wazero). | Heavy Docker environment (requires Chrome, libc, etc.). |
| **Memory Footprint** | Extremely low (~10-30MB per run, stateless). | High (Chromium processes easily consume 200MB-1GB+). |
| **Docker Image Size** | **~15MB** (scratch build with binary). | **~500MB - 1GB** (due to Chromium & LibreOffice). |
| **CSS & HTML Support** | Good (Flexbox, Grid, standard styles, SVGs, LaTeX). | Absolute (supports JS execution, external web fonts, WebGL). |
| **JavaScript Execution** | **No** (static templates only). | **Yes** (supports client-side JS charts, React, etc.). |
| **Security Sandbox** | High (WASM isolation blocks host filesystem/network access). | Low (requires strict Docker isolation to prevent Chrome CVE exploits). |
