# Wasmee Go Client Examples

This directory contains examples demonstrating how to use the `wasmee` client runner to execute guest WebAssembly modules on the standalone Rust execution server.

## Prerequisites

1. **Rust Server Running**:
   Build and start the Rust `wasmee` HTTP execution engine:
   ```bash
   # From workspace root
   cd wasmee
   cargo build --package wasmee --release
   ./target/release/wasmee
   ```

2. **Guest WASM Compiled**:
   Compile the guest WebAssembly module to WASI target:
   ```bash
   # From workspace root
   cd wasmee
   cargo build --package wasmee-guest --target wasm32-wasip1 --release
   ```

## Running Examples

### 1. In-Memory Run
Runs a simple successful guest function execution showing oplog captures:
```bash
# From workspace root
go run connectors/wasmee/examples/in-memory/main.go
```

### 2. Crash Recovery Run
Simulates a host crash during checkpointing, reload session state from the SnapshotStore, and resumes execution to completion:
```bash
# From workspace root
go run connectors/wasmee/examples/crash-recovery/main.go
```
