# GoDam developer tasks. Run `make` for the list of targets.
#
# The Vue app lives in ./frontend; npm's --prefix flag runs its scripts from
# the repo root, so no `cd` is needed.

.DEFAULT_GOAL := help

DEV_PORT ?= 5173
GOARCH ?= amd64
VERSION ?= dev

NPM := npm --prefix frontend

.PHONY: help install web dev build-web api build release tidy clean

help: ## List available targets
	@echo "GoDam targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

install: ## Install frontend dependencies
	$(NPM) ci

web: ## Run the Vite dev server (override with DEV_PORT=...)
	$(NPM) run dev -- --port $(DEV_PORT)

dev: ## Run the API and web dev server together (Ctrl+C stops both)
	@test -d frontend/node_modules || { echo "frontend deps missing: run 'make install'"; exit 1; }
	@echo "GoDam API on :8080 + web on http://localhost:$(DEV_PORT)  (Ctrl+C to stop)"
	@trap 'kill 0' INT TERM; \
		go run . & \
		$(NPM) run dev -- --port $(DEV_PORT) & \
		wait

build-web: ## Build the Vue app into frontend/dist
	@find frontend/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
	$(NPM) run build

api: ## Run the Go server (the JSON API and Vue frontend are wired in main.go)
	go run .

build: ## Build the Go binary into bin/godam
	go build -ldflags="-X main.version=$(VERSION)" -o bin/godam .

release: ## Cross-compile a static linux binary into bin/godam-linux-$(GOARCH)
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" \
		-o bin/godam-linux-$(GOARCH) .

tidy: ## Tidy Go modules
	go mod tidy

clean: ## Remove build artifacts (keeps frontend/dist/.gitkeep)
	rm -rf bin
	find frontend/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
