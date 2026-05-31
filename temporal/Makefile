.PHONY: infra-up infra-down run-worker-helloworld run-starter-helloworld run-worker-signal run-starter-signal run-worker-saga run-starter-saga run-loadtest test

# Запуск инфраструктуры (Temporal Server + PostgreSQL + Web UI)
infra-up:
	docker compose -f docker/docker-compose.yaml up -d

# Остановка инфраструктуры
infra-down:
	docker compose -f docker/docker-compose.yaml down -v

# Запуск воркера HelloWorld
run-worker-helloworld:
	go run examples/helloworld/worker/main.go

# Запуск стартера HelloWorld
run-starter-helloworld:
	go run examples/helloworld/starter/main.go

# Запуск воркера Signal & Query
run-worker-signal:
	go run examples/signal/worker/main.go

# Запуск стартера Signal & Query
run-starter-signal:
	go run examples/signal/starter/main.go

# Запуск воркера Saga
run-worker-saga:
	go run examples/saga/worker/main.go

# Запуск стартера Saga (успешный тур + компенсирующий откат)
run-starter-saga:
	go run examples/saga/starter/main.go

# Запуск нагрузочного теста (1000 процессов, конкурентность 50)
run-loadtest:
	LOAD_CONCURRENCY=50 LOAD_PROCESSES_COUNT=1000 go run examples/loadtest/main.go

# Запуск тестов модуля в виртуальной тестовой среде
test:
	go test -v ./...
