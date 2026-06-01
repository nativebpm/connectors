# Stage 1: Build the static Go host binary
FROM golang:1.26-alpine AS builder

# Install build tools needed for CGO compilation
RUN apk add --no-cache git build-base curl

WORKDIR /app/host

# Copy the host module dependency files
COPY host/go.mod host/go.sum* ./
RUN go mod download

# Copy the entire host source code (including main.go and the durable package)
COPY host/ ./

# Build static binary linked against musl libc
RUN CGO_ENABLED=1 GOOS=linux go build -tags musl -ldflags '-linkmode external -extldflags "-static"' -o host main.go

# Stage 2: Deploy in a scratch container
FROM scratch

WORKDIR /app

# Copy the static binary and the WASM worker into the container
COPY --from=builder /app/host/host /app/host
COPY worker/worker.wasm /app/worker/worker.wasm

# Set execution command
ENTRYPOINT ["/app/host"]
