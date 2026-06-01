module github.com/nativebpm/connectors/durable-wasm/examples/gotenberg-telegram/host

go 1.26

require github.com/nativebpm/connectors/durable-wasm/host v0.0.0

require github.com/bytecodealliance/wasmtime-go/v20 v20.0.0 // indirect

replace github.com/nativebpm/connectors/durable-wasm/host => ../../../host
