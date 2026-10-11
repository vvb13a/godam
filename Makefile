# GoDam developer tasks. Run `make` for the list of targets.

.DEFAULT_GOAL := help

GOARCH ?= amd64

.PHONY: help api build release tidy clean

help: ## List available targets
	@echo "GoDam targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

api: ## Run the server (the JSON API and Vue frontend are wired in main.go)
	go run .

build: ## Build the Go binary into bin/godam
	go build -o bin/godam .

release: ## Cross-compile a static linux binary into bin/godam-linux-$(GOARCH)
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -trimpath -ldflags="-s -w" \
		-o bin/godam-linux-$(GOARCH) .

tidy: ## Tidy Go modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin
