SHELL := /bin/bash
COMPOSE := docker compose
WORKER_VENV := worker/.venv

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.env:
	cp .env.example .env
	@echo "Created .env from .env.example — change the secrets before sharing this machine."

.PHONY: up
up: .env ## Build and start the whole stack
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop the stack (keeps volumes)
	$(COMPOSE) down

.PHONY: clean
clean: ## Stop the stack and delete all data volumes
	$(COMPOSE) down -v

.PHONY: ps
ps: ## Show service status
	$(COMPOSE) ps -a

.PHONY: logs
logs: ## Follow logs of api and worker nodes
	$(COMPOSE) logs -f api-1 api-2 worker-1 worker-2

.PHONY: seed
seed: ## Re-run migrations and seed data
	$(COMPOSE) up migrate seed

$(WORKER_VENV):
	python3 -m venv $(WORKER_VENV)
	$(WORKER_VENV)/bin/pip install -q uv
	cd worker && UV_PROJECT_ENVIRONMENT=.venv .venv/bin/uv sync

.PHONY: test
test: test-api test-worker ## Run unit tests

.PHONY: test-api
test-api:
	cd api && go test ./...

.PHONY: test-worker
test-worker: $(WORKER_VENV)
	cd worker && .venv/bin/pytest -q

.PHONY: lint
lint: lint-api lint-worker ## Run linters

.PHONY: lint-api
lint-api:
	cd api && test -z "$$(gofmt -l .)" && go vet ./...
	@command -v golangci-lint >/dev/null && (cd api && golangci-lint run) || echo "golangci-lint not installed, skipped"

.PHONY: lint-worker
lint-worker: $(WORKER_VENV)
	cd worker && .venv/bin/ruff check . && .venv/bin/ruff format --check . && .venv/bin/mypy
