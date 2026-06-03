# NativeBPM Connectors

Monorepository containing Go client libraries and integration connectors used across the NativeBPM platform.

## Connectors

### Core
*   **[httpstream](httpstream/)** — Flat, stream-first HTTP client designed for efficient large-file uploads using multipart streaming.

### Service
*   **[gotenberg](gotenberg/)** — Go client for Gotenberg document conversion service (HTML, Office docs, PDFs merging).
*   **[sequin](sequin/)** — Client and sync helpers for Sequin CDC and event stream platform.
*   **[telegram](telegram/)** — High-performance, stream-first Go client library for the Telegram Bot API with zero-buffer media uploads.

### Platform
*   **[nativebpm](nativebpm/)** — Public Go SDK client for interacting with the NativeBPM engine using Fluent API builders.
*   **[bpmn](bpmn/)** — Lightweight BPMN 2.0 process interpreter and DMN decision table evaluator with support for Wait States, Sagas, and WebAssembly workers.
*   **[camunda](camunda/)** — Client library for Camunda BPMN workflow engine with highly optimized task locking and Postgres CDC integration.
*   **[wasman](wasman/)** — Reusable Durable Execution Engine for running fault-tolerant Go/TinyGo business logic in an isolated WASM sandbox with O(1) memory consumption and transparent checkpointing.
*   **[wasman/runner](wasman/runner/)** — Light-weight guest-side Runner API and Fluent Workflow builder for WASM workers, completely decoupled from host dependencies.
*   **[temporal](temporal/)** — Wrapper client, worker setup configurations, and orchestration utilities for Temporal.io.

## Development

The project uses Go Workspaces to manage multiple modules.

### Prerequisites

*   Go 1.26+
*   `golangci-lint` (can be installed via `make lint-install`)

### Useful Commands

*   `make tidy` — Tidy up dependencies and sync Go workspace.
*   `make lint` — Run `golangci-lint` across all module directories.
*   `make tag` — Create and push an annotated release tag for a specific module (e.g. `gotenberg/v8.33.1`).

## License

MIT — see [LICENSE](LICENSE).

