# Wasmee Go Client Connector

This package provides a Go client for communicating with the Rust-based Wasmee sandboxed WebAssembly execution daemon.

## Protobuf Code Generation

The Go protobuf code in `pb/wasmee.pb.go` is generated from the central schema file [wasmee.proto](file:///Users/user/github.com/nativebpm/wasmee/wasmee.proto) located in the `wasmee` repository.

### Prerequisites

Make sure you have `protoc` and `protoc-gen-go` installed on your system:
```bash
# Verify protoc is installed
protoc --version

# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Regenerating Code

To regenerate the protobuf bindings, run the following command from this directory:
```bash
go generate
```

Or run `protoc` directly:
```bash
protoc --proto_path=. --go_out=pb --go_opt=paths=source_relative wasmee.proto
```
