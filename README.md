
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

You can add a module manually or use the helper target in the repository `Makefile`.

Manual steps

1. Create a folder for the module at the repository root, e.g. `./mymodule/`.
2. Initialize a module inside that folder:

	```sh
	go mod init github.com/nativebpm/connectors/mymodule
	```

3. Add the new module to the repository workspace so tools and the workspace-aware Go commands see it:

	```sh
	go work use ./mymodule
	```

4. Add a `README.md` in the module with usage examples.
5. Add an entry for the new module to this top-level `README.md` following the template above to make it easy to discover.

Using the Makefile helper

There is a convenience target `make mod` that automates most of the steps above. Run it from the repository root and enter the module name when prompted:

```sh
make mod
# Enter module name: mymodule
```

What `make mod` does

- Prompts for a module name (the directory to create).
- Creates the module directory and runs `go mod init github.com/nativebpm/connectors/<module>`.
- Sets the Go version in the new module (`GO_VERSION` in the `Makefile`, default: 1.21).
- Writes a minimal `<module>/<module>.go` file with `package <module>` so the module compiles.
- Adds the new module to the repository Go workspace with `go work use ./<module>`.

You can override the Go version used by `make mod` by passing `GO_VERSION` on the command line, for example:

```sh
make GO_VERSION=1.22 mod
```

Note: `make mod` is interactive and reads the module name from stdin. If you need a non-interactive flow, create the folder and run `go mod init` and `go work use` manually as shown above.

## License

MIT — see the [`LICENSE`](LICENSE) file.
