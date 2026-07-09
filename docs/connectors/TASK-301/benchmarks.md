# TASK-301 Memory & Performance Profiling

## Overview
This document registers the benchmark results and memory profiling for `BenchmarkRequest_JSON` and `BenchmarkRequest_JSONWithTimeout` before and after the optimization of JSON payload serialization.

## Benchmark Results Comparison

| Benchmark | Type | Time (ns/op) | Memory (B/op) | Allocations (allocs/op) |
| :--- | :--- | :---: | :---: | :---: |
| `BenchmarkRequest_JSON` | **Baseline** | 30,872 | 7,207 | 85 |
| `BenchmarkRequest_JSON` | **Optimized** | 28,652 | 6,741 | 80 |
| **Improvement** | | **-7.2%** | **-6.5%** | **-5.9%** |
| `BenchmarkRequest_JSONWithTimeout` | **Baseline** | 32,546 | 8,225 | 94 |
| `BenchmarkRequest_JSONWithTimeout` | **Optimized** | 29,897 | 7,777 | 89 |
| **Improvement** | | **-8.1%** | **-5.4%** | **-5.3%** |

## Profiling Analysis
Using `pprof -alloc_space` on the optimized build:
- The custom serialization overhead is now minimal. `encoding/json.Marshal` accounts for only **~4.5%** of all allocated memory during the benchmark.
- The remaining memory allocations (e.g. `6,741 B/op` and `80 allocs/op`) represent the baseline HTTP request-response cycle and connection management overhead inherent in `net/http` and `httptest.NewServer`.
- Specifically, the baseline request with no payload (`BenchmarkRequest_Simple`) allocates `6,064 B/op` and `68 allocs/op`, meaning that JSON serialization now introduces only `677 B/op` and `12 allocs/op` of absolute overhead.

## Key Improvements
1. **Goroutine Elimination**: Spawning one goroutine per request was completely removed, saving context-switching overhead and goroutine stack space.
2. **Pipe Elimination**: `io.Pipe()` (which allocates a mutex and channel-like synchronization structures) was removed.
3. **Explicit ContentLength**: By serializing up-front, we determine the exact payload length and set `r.Request.ContentLength`. This allows the HTTP Client to avoid chunked transfer encoding, reducing network packet framing and internal buffer allocations.
