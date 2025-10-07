
# Connectors


A collection of Go modules providing connectors and utilities for different kinds of integrations (HTTP clients, document conversion, storage adapters, webhooks, etc.).

This repository is a monorepo (Go workspace): each module lives in its own subdirectory and has its own `go.mod`.

## Modules (navigation and categories)

Below is a categorized list of modules in this repository. Each entry includes a short description, the module path for `go get`, and a link to the module folder/documentation.

### Utilities

- **httpclient** — a stream-first HTTP client for Go. Efficient multipart uploads using `io.Pipe` and composable middleware.

	- Path: `httpclient/`
	- Install: `go get github.com/nativebpm/connectors/httpclient`
	- Docs: [httpclient/README.md](httpclient/README.md)

### Service integrations

- **gotenberg** — a client for the Gotenberg document conversion service (convert URLs, HTML, Office documents to PDF, with webhook support).

	- Path: `gotenberg/`
	- Install: `go get github.com/nativebpm/connectors/gotenberg`
	- Docs: [gotenberg/README.md](gotenberg/README.md)

> More modules of different types will be added over time (not only HTTP). When adding a module, pick or create a category that best describes it.

## How to add a new module

### Quick start with Makefile

Run this command and follow the prompts:

- `make mod` - create new module with `go mod init` and `go work use`
- `make tag` — create new tags for module
- `make lint-install` — install golangci-lint
- `make lint` — run linter on all modules

### Manual setup

If you prefer to set up manually:

1. Create module directory: `mkdir mymodule`
2. Initialize module: `go mod init github.com/nativebpm/connectors/mymodule`
3. Add to workspace: `go work use ./mymodule`
4. Add `README.md` and update this file's module list

## License

MIT — see the [`LICENSE`](LICENSE) file.
