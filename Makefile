GO_VERSION ?= 1.21

mod:
	@read -p "Enter module name: " MODULE; \
	mkdir -p $$MODULE; \
	go -C $$MODULE mod init github.com/nativebpm/connectors/$$MODULE; \
	go -C $$MODULE mod edit -go=$(GO_VERSION); \
	echo "package $$MODULE" > $$MODULE/$$MODULE.go; \
	go work use ./$$MODULE

tag:
	@read -p "Enter module name: " MODULE; \
	echo ""; \
	echo "Last 3 tags for $$MODULE:"; \
	git for-each-ref --sort=-creatordate --format='%(refname:short) - %(contents:subject)' refs/tags | grep "^$$MODULE/" | head -n 3 || echo "No tags found"; \
	echo ""; \
	read -p "Enter version (vX.Y.Z): " VERSION; \
	read -p "Enter message: " MESSAGE; \
	TAG_NAME="$$MODULE/$$VERSION"; \
	git tag -a "$$TAG_NAME" -m "$$VERSION ($$MESSAGE)"; \
	git push --tags; \
	echo "Tag $$TAG_NAME created and pushed"

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

lint:
	golangci-lint version
	golangci-lint cache clean
	find . -name go.mod -exec dirname {} \; | xargs -I {} golangci-lint run {}