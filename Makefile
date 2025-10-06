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