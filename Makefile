.PHONY: up down logs build restart test lint seed migrate clean help security-check security-check-backend security-check-frontend

# Default target
.DEFAULT_GOAL := help

# Load .env if exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

## ── Docker ────────────────────────────────────────────────────────────
up: ## Start all services
	docker compose up -d

up-build: ## Rebuild and start all services
	docker compose up -d --build

down: ## Stop all services
	docker compose down

down-v: ## Stop all services and remove volumes
	docker compose down -v

logs: ## Tail logs from all services
	docker compose logs -f

logs-backend: ## Tail backend logs
	docker compose logs -f backend

logs-frontend: ## Tail frontend logs
	docker compose logs -f frontend

logs-mysql: ## Tail MySQL logs
	docker compose logs -f mysql

restart: ## Restart all services
	docker compose restart

restart-backend: ## Restart only backend
	docker compose restart backend

build: ## Build all Docker images
	docker compose build

## ── Database ───────────────────────────────────────────────────────────
migrate: ## Run database migrations
	docker compose exec backend go run cmd/migrate/main.go

seed: ## Seed database with test data
	docker compose exec backend go run cmd/seed/main.go

db-shell: ## Open MySQL shell
	docker compose exec mysql mysql -u $(MYSQL_USER) -p$(MYSQL_PASSWORD) $(MYSQL_DATABASE)

db-root: ## Open MySQL shell as root
	docker compose exec mysql mysql -u root -p$(MYSQL_ROOT_PASSWORD)

## ── Backend ────────────────────────────────────────────────────────────
backend-shell: ## Shell into backend container
	docker compose exec backend sh

backend-test: ## Run backend tests
	docker compose exec backend go test ./...

backend-lint: ## Lint backend code
	docker compose exec backend golangci-lint run ./...

backend-fmt: ## Format backend code
	docker compose exec backend go fmt ./...

## ── Frontend ───────────────────────────────────────────────────────────
frontend-shell: ## Shell into frontend container
	docker compose exec frontend sh

frontend-test: ## Run frontend tests
	docker compose exec frontend bun run test

frontend-lint: ## Lint frontend code
	docker compose exec frontend bun run lint

frontend-fmt: ## Format frontend code
	docker compose exec frontend bun run format

## ── Local dev (without Docker) ─────────────────────────────────────────
dev-backend: ## Run backend locally with Air
	cd backend && air

dev-frontend: ## Run frontend locally
	cd frontend && bun run dev

## ── Quality ────────────────────────────────────────────────────────────
test: backend-test frontend-test ## Run all tests

lint: backend-lint frontend-lint ## Run all linters

## ── Security ───────────────────────────────────────────────────────────
security-check-backend: ## Run govulncheck on backend
	docker compose exec backend go install golang.org/x/vuln/cmd/govulncheck@latest
	docker compose exec backend govulncheck ./...

security-check-frontend: ## Run npm audit on frontend
	docker compose exec frontend npm audit --audit-level=high

security-check: security-check-backend security-check-frontend ## Run all security vulnerability scans

## ── Setup ──────────────────────────────────────────────────────────────
setup: ## First-time setup (copy .env, create dirs)
	@if [ ! -f .env ]; then cp .env.example .env && echo "Created .env from .env.example"; fi
	@mkdir -p docker/mysql docker/redis

clean: ## Remove all containers, volumes, and build artifacts
	docker compose down -v --remove-orphans
	docker system prune -f

## ── Health ─────────────────────────────────────────────────────────────
health: ## Check backend health endpoint
	curl -s http://localhost:$(BACKEND_PORT)/health | jq .

## ── Help ───────────────────────────────────────────────────────────────
help: ## Show this help
	@echo "trader-claude Makefile"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 4) } ' $(MAKEFILE_LIST)
