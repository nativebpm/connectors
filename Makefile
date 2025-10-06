# Usage:
#   make mod

GO_VERSION ?= 1.21

mod:
	@read -p "Enter module name: " MODULE; \
	if [ -z "$$MODULE" ]; then \
		echo "Module name cannot be empty"; \
		exit 1; \
	fi; \
	if [ -d "$$MODULE" ]; then \
		echo "Module '$$MODULE' already exists"; \
		exit 1; \
	fi; \
	echo "Creating module: $$MODULE"; \
	mkdir -p $$MODULE; \
	go -C $$MODULE mod init github.com/nativebpm/connectors/$$MODULE; \
	go -C $$MODULE mod edit -go=$(GO_VERSION); \
	echo "package $$MODULE" > $$MODULE/$$MODULE.go; \
	go work use ./$$MODULE; \
	echo "Module '$$MODULE' created and added to workspace"

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

lint:
	golangci-lint version
	golangci-lint cache clean
	find . -name go.mod -exec dirname {} \; | xargs -I {} golangci-lint run {}

gotenberg-run:
	docker run \
		-p 3000:3000 \
		--name gotenberg \
		--add-host="host.docker.internal:host-gateway" \
		gotenberg/gotenberg:8

gotenberg:
	docker start gotenberg
