export GOWORK := $(abspath $(CURDIR)/../../go.work)

.PHONY: wasm
wasm:
	tinygo build -o jsonschema.wasm -target=wasi ./cmd/jsonschema/
	mkdir -p ../../platform/sdks/typescript/dist
	cp jsonschema.wasm ../../platform/sdks/typescript/dist/jsonschema.wasm
	mkdir -p ../../platform/sdks/python/nativebpm
	cp jsonschema.wasm ../../platform/sdks/python/nativebpm/jsonschema.wasm
	brotli -q 11 -f jsonschema.wasm
	rm jsonschema.wasm
