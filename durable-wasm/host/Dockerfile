# Stage 1: Build the static Go host binary
FROM golang:1.26-alpine AS builder

# Install build tools needed for CGO compilation
RUN apk add --no-cache git build-base curl

WORKDIR /app

# Copy the host module files
COPY host/go.mod ./host/
# Copy go.sum if it exists (using a wildcard so it doesn't fail if go.sum isn't generated yet)
COPY host/go.sum* ./host/

WORKDIR /app/host
RUN go mod download

# Copy host source code
COPY host/main.go ./host/

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
