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

Ensure you have `k6` installed. Start the server in one terminal:

```bash
go run examples/server/main.go --addr :8080
```

In another terminal, run the load test:

```bash
k6 run examples/k6/load_test.js
```
