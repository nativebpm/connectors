# Stage 1: Build the Go host binary
FROM golang:1.26-bookworm AS builder

RUN apt-get update && apt-get install -y gcc g++ make git curl libc6-dev

WORKDIR /app

# Copy workspace configuration and local modules
COPY go.work go.work.sum ./
COPY durable-wasm durable-wasm
COPY httpstream httpstream

# Build the host application
WORKDIR /app/durable-wasm/examples/simple/host
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags '-extldflags "-static"' -o host main.go

# Stage 2: Setup execution environment with Litestream
FROM litestream/litestream:latest AS litestream-bin

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite curl

WORKDIR /app

# Copy Litestream binary
COPY --from=litestream-bin /usr/local/bin/litestream /usr/local/bin/litestream

# Copy application binary and WASM worker
COPY --from=builder /app/durable-wasm/examples/simple/host/host /app/host
COPY durable-wasm/examples/simple/worker/worker.wasm /app/worker/worker.wasm

# Copy litestream configuration and entrypoint
COPY durable-wasm/examples/simple/litestream.yml /etc/litestream.yml
COPY durable-wasm/examples/simple/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
