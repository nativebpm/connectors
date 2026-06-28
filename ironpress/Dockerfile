# Stage 1: Build the Rust ironpress binary
FROM rust:1.80-slim AS rust-builder
WORKDIR /app
RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
RUN cargo install --git https://github.com/gastongouron/ironpress.git --root /usr/local

# Stage 2: Build the Go server wrapper
FROM golang:1.23-alpine AS go-builder
WORKDIR /app
COPY go.mod ./
# Note: we disable proxy or use normal mod download as go.sum is not present or contains workspace refs
RUN go env -w GOPROXY=direct
RUN go mod init github.com/nativebpm/connectors/ironpress || true
RUN go mod edit -require=github.com/nativebpm/httpstream@v0.0.4 || true
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server examples/server/main.go

# Stage 3: Final runtime image
FROM debian:bookworm-slim
WORKDIR /app
# Install runtime dependencies for weasyprint/ironpress (pango, cairo, glib)
RUN apt-get update && apt-get install -y \
    shared-mime-info \
    libcairo2 \
    libpango-1.0-0 \
    libpangocairo-1.0-0 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=rust-builder /usr/local/bin/ironpress /usr/local/bin/ironpress
COPY --from=go-builder /app/server /app/server

EXPOSE 8080
ENTRYPOINT ["/app/server", "--addr", ":8080", "--bin", "/usr/local/bin/ironpress"]
