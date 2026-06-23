# Gotenberg Client Benchmarks

This file documents the benchmark results and memory footprint of the updated Gotenberg client library, focusing on the new builder configuration chain for Gotenberg v8.34.0.

## Environment
- **OS**: macOS (darwin)
- **Architecture**: arm64 (Apple M5)
- **Go Version**: go1.26.1

## Benchmark Results

Run Command: `go test -v -bench=. -benchmem`

```
goos: darwin
goarch: arm64
pkg: github.com/nativebpm/connectors/gotenberg/v8
cpu: Apple M5
BenchmarkChromiumBuilder-10      	 2289012	       530.0 ns/op	    2096 B/op	      11 allocs/op
BenchmarkPDFEnginesBuilder-10    	 2374602	       536.1 ns/op	    2088 B/op	      11 allocs/op
```

## Analysis

- **BenchmarkChromiumBuilder**: Creating a new Chromium conversion builder and chaining 4 security fields takes **530 ns** per operation, allocating **2.1 KB** across **11 allocations**.
- **BenchmarkPDFEnginesBuilder**: Creating a PDFEngines conversion builder and chaining 3 Factur-X configuration fields takes **536 ns** per operation, allocating **2.08 KB** across **11 allocations**.

The memory allocations are minimal and primarily reflect:
1. Re-instantiating the internal `Multipart` request builder.
2. Allocating maps/slices inside the multi-part request body.

No memory leaks or hot paths were identified.
