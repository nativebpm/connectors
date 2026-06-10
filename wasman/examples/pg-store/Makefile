.PHONY: all build build-worker run clean run-docker

all: build

build: build-worker

build-worker:
	@echo "Building WASM worker for pg-store example using TinyGo..."
	tinygo build -o worker/worker.wasm -target=wasi worker/main.go

run: build-worker
	@echo "Running pg-store example host..."
	cd host && go run main.go

run-docker: build-worker
	@echo "Running local PostgreSQL on port 5435 and starting the pg-store demonstration..."
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	sleep 5
	cd host && go run main.go
	docker compose down

clean:
	@echo "Cleaning pg-store example artifacts..."
	rm -f worker/worker.wasm
