.PHONY: all build build-worker build-host run clean test docker-build

all: build

build: build-worker build-host

build-worker:
	@echo "Building WASM worker using TinyGo..."
	tinygo build -o examples/simple/worker/worker.wasm -target=wasi examples/simple/worker/main.go

build-host:
	@echo "Building Go host orchestrator..."
	cd examples/simple/host && go build -o host main.go

run:
	make -C examples/simple run

test: build-worker
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning artifacts..."
	rm -f examples/simple/worker/worker.wasm examples/simple/host/host
	make -C examples/simple clean
	make -C examples/process-csv clean
	make -C examples/temporal clean
	make -C examples/camunda clean
	make -C examples/gotenberg-telegram clean

run-csv-example:
	make -C examples/process-csv run

run-temporal-example:
	make -C examples/temporal run

run-camunda-example:
	make -C examples/camunda run

run-gotenberg-telegram-example:
	make -C examples/gotenberg-telegram run

run-simple-docker:
	make -C examples/simple run-docker

docker-build:

	@echo "Building scratch-based Docker image..."
	docker build -t wasm-durable-host -f examples/simple/host/Dockerfile .

