.PHONY: dev build clean install-tools

MINIFY := $(shell go env GOPATH)/bin/minify

# Development: run with original HTML files
# Usage: make dev [PORT=9090]
PORT ?= 8080
dev:
	go run . -port $(PORT)

# Production build: minify HTML temporarily, build binary, restore originals
build:
	@echo "Backing up original files..."
	@cp web/index.html web/index.html.orig
	@cp web/rules.html web/rules.html.orig
	@echo "Minifying web assets..."
	@$(MINIFY) --type=html -o web/index.min.html web/index.html
	@$(MINIFY) --type=html -o web/rules.min.html web/rules.html
	@mv web/index.min.html web/index.html
	@mv web/rules.min.html web/rules.html
	@echo "Building binary..."
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o hooklab .
	@echo "Restoring original files..."
	@mv web/index.html.orig web/index.html
	@mv web/rules.html.orig web/rules.html
	@echo "Build complete: ./hooklab"

# Clean build artifacts
clean:
	@rm -f hooklab

# Install minify tool (run once)
install-tools:
	go install github.com/tdewolff/minify/v2/cmd/minify@latest
