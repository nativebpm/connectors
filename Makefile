GO_VERSION ?= 1.21

mod:
	@read -p "Enter module name: " MODULE; \
	mkdir -p $$MODULE; \
	go -C $$MODULE mod init github.com/nativebpm/connectors/$$MODULE; \
	go -C $$MODULE mod edit -go=$(GO_VERSION); \
	echo "package $$MODULE" > $$MODULE/$$MODULE.go; \
	go work use ./$$MODULE

tidy:
	find . -name go.mod -execdir go mod tidy \;
	go work sync

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

tag-list:
	@read -p "Enter module name: " MODULE; \
	echo ""; \
	echo "Tags for $$MODULE:"; \
	git for-each-ref --sort=-creatordate --format='%(refname:short) - %(contents:subject)' refs/tags | grep "^$$MODULE/" || echo "No tags found"; \
	echo ""

tag-del:
	@read -p "Enter module name: " MODULE; \
	read -p "Enter version to delete (vX.Y.Z): " VERSION; \
	TAG_NAME="$$MODULE/$$VERSION"; \
	git tag -d "$$TAG_NAME" || echo "Tag $$TAG_NAME not found locally"; \
	git push --delete origin "$$TAG_NAME" || echo "Tag $$TAG_NAME not found on remote"; \
	echo "Tag $$TAG_NAME deleted if it existed"

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

lint:
	golangci-lint version
	golangci-lint cache clean
	find . -name go.mod -exec dirname {} \; | xargs -I {} golangci-lint run {}