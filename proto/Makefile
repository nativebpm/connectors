.PHONY: install generate clean help

# Helper to output available commands
help:
	@echo "Available commands:"
	@echo "  make install  - Install buf and required Go protoc plugins locally"
	@echo "  make generate - Fetch dependencies (buf dep update) and generate Go/Connect code & OpenAPI spec"
	@echo "  make clean    - Remove generated files (pb.go, connect.go, openapi.yaml)"

# Install all necessary binaries and plugins to GOPATH/bin
install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
	go install github.com/bufbuild/buf/cmd/buf@latest

# Run dependency update and code generation using locally installed buf
generate:
	export PATH="$$(go env GOPATH)/bin:$$PATH" && buf dep update && buf generate

# Clean up generated stubs and OpenAPI schemas
clean:
	rm -f openapi.yaml
	rm -f nativebpm/v1/workflow.pb.go
	rm -rf nativebpm/v1/nativebpmpbconnect
