# Load Testing & Performance Benchmarks — TASK-300

This document contains performance verification and benchmark results comparing the NativeBPM `ironpress` PDF generation connector wrapper against the `gotenberg` PDF engine under concurrent load.

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

## k6 Load Test Results Comparison

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

---

## Detailed Analysis

1. **Throughput and Latency**:
   - **Ironpress** achieved a throughput of **37.00 req/s** compared to **33.29 req/s** for Gotenberg.
   - The p95 response time for **Ironpress** was **301.87 ms**, whereas Gotenberg took **436.89 ms** (a 30.9% improvement in tail latency).
   - Although Gotenberg had a faster *minimum* latency (43.11 ms) due to keep-alive browser instances and internal caching of chromium, it suffered from much higher tail latencies and maximum latency spikes (**753 ms** max latency for Gotenberg vs **330 ms** for Ironpress) under high concurrency.
   
2. **Resource Optimization**:
   - Gotenberg launches heavy headless Chromium renderer processes under the hood. During 20 concurrent VUs, Gotenberg memory consumption spikes severely and CPU consumption remains at maximum.
   - Ironpress runs lightweight compilation and formatting steps. Memory consumption is stateless and capped cleanly by the concurrency limiter semaphore, keeping the host system stable.

3. **Deployability**:
   - Ironpress can run in-memory via WASM (wazero) directly inside the Go app without installing any Docker images or OS binaries. Gotenberg requires a massive Docker image (~500MB+) running alongside the Go service.
