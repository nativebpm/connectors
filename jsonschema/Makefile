export GOWORK := $(abspath $(CURDIR)/../../go.work)

.PHONY: wasm
wasm:
	GOOS=wasip1 GOARCH=wasm go build -o jsonschema.wasm ./cmd/jsonschema/
	brotli -q 11 -f jsonschema.wasm
