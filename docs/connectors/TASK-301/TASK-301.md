---
task: TASK-301
status: Completed
summary: Refactor JSON payload serialization in HTTP requests to optimize memory allocations and avoid redundant piping.
---

# TASK-301: Optimize JSON payload serialization in httpstream

## Problem Statement
In `httpstream.go`/`request.go`, `BenchmarkRequest_JSON` allocates a high amount of memory per operation (approx. 7200 B/op and 85 allocs/op).
Every request using JSON payload currently creates an asynchronous `io.Pipe`, starts a new goroutine, and serializes the JSON data on-the-fly using `json.NewEncoder(pw)`.

For standard requests that don't need to stream data chunk-by-chunk dynamically (or can be pre-serialized), setting up a piping mechanism and goroutine per request is extremely heavy and introduces latency, CPU context switching, and unnecessary heap allocations.

## Objectives
1. Optimize the JSON serialization in `Request.Send()`.
2. Avoid redundant piping or allocate a static buffer/pool if appropriate to lower B/op.
3. Verify that the benchmarks (`BenchmarkRequest_JSON`, `BenchmarkRequest_JSONWithTimeout`, and others) compile and run with reduced memory allocations.
