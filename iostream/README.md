# iostream: Zero-Allocation Fluent Streaming Helper for Go

`iostream` (`github.com/nativebpm/connectors/iostream`) encapsulates the complexity of Go's `io.Pipe()`, background writer goroutines, `CloseWithError` error propagation, and HTTP/Storage streaming into a clean, intuitive Fluent API.

## Features

- **Encapsulated `io.Pipe` Orchestration**: Handles goroutine creation, cleanup, and error propagation internally.
- **Fluent Stream Builder (`StreamBuilder`)**: Method chaining for JSON encoding, custom stream writers, HTTP request dispatching, and header injection.
- **Zero Heap Buffering**: Streams payload bytes directly from encoder to HTTP/Storage reader without allocating giant `[]byte` buffers in RAM.

---

## Usage Example

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/nativebpm/connectors/iostream"
)

type ProcessVariables struct {
	ProcessInstanceID string         `json:"process_instance_id"`
	Variables         map[string]any `json:"variables"`
}

func main() {
	payload := ProcessVariables{
		ProcessInstanceID: "proc-1001",
		Variables:         map[string]any{"status": "APPROVED", "score": 98.5},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Single-line Fluent Streaming HTTP Request
	resp, err := iostream.NewStream().
		WithJSONPayload(payload).
		ToURL(http.MethodPost, "https://api.nativebpm.cloud/v1/execution/variables").
		WithHeader("Authorization", "Bearer glpat-token-secret").
		ExecuteHTTPRequest(ctx)

	if err != nil {
		log.Fatalf("Streaming request failed: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Response status: %d", resp.StatusCode)
}
```
