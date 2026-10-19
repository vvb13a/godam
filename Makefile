# GoDam developer tasks. Run `make` for the list of targets.
#
# The Vue app lives in ./frontend; npm's --prefix flag runs its scripts from
# the repo root, so no `cd` is needed.

.DEFAULT_GOAL := help

# Capture this file before -include adds .env to MAKEFILE_LIST (the help target
# greps only this file).
THIS_MAKEFILE := $(firstword $(MAKEFILE_LIST))

# Load local stack settings (domain + imgproxy keys) so a host-run godam
# matches the Docker Caddy/imgproxy stack.
-include .env

DEV_PORT ?= 5173
DEV_INTERNAL_URL ?= http://host.docker.internal:8080
DEV_IMGPROXY_URL ?= http://127.0.0.1:8081
DEV_CONVERSION_DIR ?= $(CURDIR)/.dev/conversions
GODAM_DOMAIN ?= localhost
GOARCH ?= amd64
VERSION ?= dev

NPM := npm --prefix frontend
COMPOSE_DEV := docker compose -f docker-compose.yml -f docker-compose.dev.yml

.PHONY: help install web dev dev-stack dev-stack-down dev-stack-reload build-web api build release tidy clean

help: ## List available targets
	@echo "GoDam targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(THIS_MAKEFILE) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

install: ## Install frontend dependencies
	$(NPM) ci

web: ## Run the Vite dev server (override with DEV_PORT=...)
	$(NPM) run dev -- --port $(DEV_PORT)

dev-stack: ## Start Caddy + imgproxy for hybrid host development
	@mkdir -p $(DEV_CONVERSION_DIR)
	$(COMPOSE_DEV) up -d --no-deps caddy imgproxy

dev-stack-down: ## Stop the dev Caddy + imgproxy containers
	$(COMPOSE_DEV) stop caddy imgproxy

dev-stack-reload: ## Restart Caddy after editing the Caddyfile
	$(COMPOSE_DEV) restart caddy

dev: ## Run the API + Vite on the host against the Caddy/imgproxy stack
	@test -d frontend/node_modules || { echo "frontend deps missing: run 'make install'"; exit 1; }
	@mkdir -p $(DEV_CONVERSION_DIR)
	@echo "GoDam API :8080, Vite :$(DEV_PORT), Caddy https://$(GODAM_DOMAIN)"
	@trap 'kill 0' INT TERM; \
		GODAM_DOMAIN="$(GODAM_DOMAIN)" \
		GODAM_CONVERSION_DIR="$(DEV_CONVERSION_DIR)" \
		GODAM_INTERNAL_URL="$(DEV_INTERNAL_URL)" \
		GODAM_IMGPROXY_URL="$(DEV_IMGPROXY_URL)" \
		GODAM_IMGPROXY_KEY="$(IMGPROXY_KEY)" \
		GODAM_IMGPROXY_SALT="$(IMGPROXY_SALT)" \
		go run . & \
		$(NPM) run dev -- --port $(DEV_PORT) --host 0.0.0.0 & \
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
