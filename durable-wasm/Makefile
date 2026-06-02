.PHONY: all build build-worker build-host run clean test docker-build

all: build

build: build-worker build-host

build-worker:
	@echo "Building WASM worker using TinyGo..."
	tinygo build -o examples/durable-s3/worker/worker.wasm -target=wasi examples/durable-s3/worker/main.go

build-host:
	@echo "Building Go host orchestrator..."
	cd examples/durable-s3/host && go build -o host main.go

run:
	make -C examples/durable-s3 run

test: build-worker
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning artifacts..."
	rm -f examples/durable-s3/worker/worker.wasm examples/durable-s3/host/host
	make -C examples/durable-s3 clean
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

run-durable-s3-docker:
	make -C examples/durable-s3 run-docker

docker-build:

	@echo "Building scratch-based Docker image..."
	docker build -t wasm-durable-host -f examples/durable-s3/host/Dockerfile .

