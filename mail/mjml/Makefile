.PHONY: build test clean

# Default target
all: build

# Build the Rust compiler into WASM using Docker and compress with Brotli
build:
	@echo "==> Building Rust WASM compiler via Docker..."
	docker run --rm \
		-v $$(pwd)/../../..:/workspace \
		-w /workspace/connectors/mail/mjml/mrml-compiler \
		rust:latest \
		sh -c "rustup target add wasm32-wasip1 && cargo build --target wasm32-wasip1 --release"
	@echo "==> Compressing WASM binary with Brotli..."
	go run mrml-compiler/compress.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf mrml-compiler/target
