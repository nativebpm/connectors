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

release:
	@read -p "Enter module name: " MODULE; \
	read -p "Enter version (e.g., v0.0.1): " VERSION; \
	read -p "Enter release message: " MESSAGE; \
	if [ -z "$$MODULE" ] || [ -z "$$VERSION" ] || [ -z "$$MESSAGE" ]; then \
		echo "Error: Module name, version, and message are required"; \
		exit 1; \
	fi; \
	if [ ! -d "$$MODULE" ]; then \
		echo "Error: Module '$$MODULE' does not exist"; \
		exit 1; \
	fi; \
	if ! echo "$$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "Error: Version must be in format vX.Y.Z (e.g., v0.0.1)"; \
		exit 1; \
	fi; \
	TAG_NAME="$$MODULE/$$VERSION"; \
	echo "Creating annotated tag: $$TAG_NAME"; \
	echo "Message: $$MESSAGE"; \
	git tag -a "$$TAG_NAME" -m "$$MESSAGE"; \
	echo "Pushing tag to origin..."; \
	git push --tags; \
	echo "Release $$TAG_NAME created and pushed successfully"

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

lint:
	golangci-lint version
	golangci-lint cache clean
	find . -name go.mod -exec dirname {} \; | xargs -I {} golangci-lint run {}