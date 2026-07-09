# Implementation Plan - TASK-301: Optimize JSON payload serialization

We will refactor the JSON serialization path in `Request.Send()` to avoid spawning a goroutine and using `io.Pipe()`.

## Proposed Refactoring Options

### Option A: Direct Serialization via `json.Marshal`
Instead of using `io.Pipe()` and a goroutine:
1. Marshal `r.body.content` directly to a byte slice using `json.Marshal`.
2. Wrap the byte slice in a `bytes.NewReader`.
3. Set `r.Request.Body = io.NopCloser(reader)` and `r.Request.ContentLength = int64(len(data))`.

*Pros:*
- Eliminates goroutine spawning per request (huge reduction in CPU and memory overhead).
- Eliminates `io.Pipe` allocation.
- Sets `ContentLength` explicitly, allowing the Go HTTP client to avoid chunked transfer encoding, saving network and buffer overhead.
- Extremely simple and robust.

*Cons:*
- Allocates the marshaled byte slice.

### Option B: Pooled Buffer Serialization
1. Use a `sync.Pool` of `*bytes.Buffer`.
2. Retrieve a buffer, serialize using `json.NewEncoder(buf).Encode(...)`.
3. Wrap in a custom `io.ReadCloser` that returns the buffer to the pool on `Close()`.

*Pros:*
- Reuses buffer memory, potentially lower allocations.

*Cons:*
- `json.NewEncoder` may still allocate internal structures.
- More complex implementation, potential buffer recycling bugs.
- If body is not closed (e.g. in some edge cases), the buffer won't return to the pool (though it will be GCed).

## Step-by-Step Execution Plan
1. Create `task.md` outlining the checklist.
2. Implement Option A first and run the benchmarks.
3. Compare the benchmark results with the baseline.
4. If Option A is sufficiently fast and low-alloc, finalize it. Otherwise, implement and benchmark Option B.
5. Create `walkthrough.md` with final results.
6. Commit and push the changes.
