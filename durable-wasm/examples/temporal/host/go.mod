module github.com/nativebpm/connectors/durable-wasm/examples/temporal/host

go 1.26

require github.com/nativebpm/connectors/durable-wasm v0.0.0

require (
	github.com/bytecodealliance/wasmtime-go/v20 v20.0.0 // indirect
	github.com/nativebpm/httpstream v0.0.3 // indirect
)

replace github.com/nativebpm/connectors/durable-wasm => ../../../
