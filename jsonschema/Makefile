export GOWORK := $(abspath $(CURDIR)/../../go.work)

.PHONY: wasm
wasm:
	tinygo build -o jsonschema.wasm -target=wasi ./cmd/jsonschema/
	brotli -q 11 -f jsonschema.wasm
	rm jsonschema.wasm
