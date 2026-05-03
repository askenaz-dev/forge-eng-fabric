# Forge Engineering Fabric — root Makefile
# Targets are designed to work on Windows (pwsh) and Unix shells where reasonable.
# On Windows, run via `make` from Git Bash, WSL, or use `pwsh` equivalents.

SHELL := /bin/sh
COMPOSE := docker compose -f deploy/compose/docker-compose.yaml

.PHONY: help bootstrap lint test up down logs ps smoke clean fmt

help:
	@echo "Forge — make targets:"
	@echo "  bootstrap   Install local toolchain prerequisites (info only)"
	@echo "  lint        Run linters across Go / Python / Node"
	@echo "  test        Run tests across Go / Python / Node"
	@echo "  up          Bring docker-compose stack up (detached)"
	@echo "  down        Stop docker-compose stack"
	@echo "  logs        Tail docker-compose logs"
	@echo "  ps          Show docker-compose processes"
	@echo "  smoke       Run end-to-end smoke test against local stack"
	@echo "  fmt         Format code across languages"
	@echo "  clean       Remove generated artifacts"

bootstrap:
	@echo "Required toolchain:"
	@echo "  - Docker Desktop (compose v2)"
	@echo "  - Go >= 1.22"
	@echo "  - Node.js >= 20 + pnpm"
	@echo "  - Python >= 3.12 + uv (recommended)"
	@echo ""
	@echo "Verify with:"
	@echo "  docker --version && go version && node -v && pnpm -v && python --version"

lint:
	@echo ">> Go vet"
	@cd services/control-plane && go vet ./... || true
	@cd services/registry && go vet ./... || true
	@cd services/audit && go vet ./... || true
	@echo ">> Python ruff"
	@cd services/alfred && ruff check . || true
	@echo ">> Node lint"
	@cd portal && pnpm lint || true

test:
	@echo ">> Go tests"
	@cd services/control-plane && go test ./... || true
	@cd services/registry && go test ./... || true
	@cd services/audit && go test ./... || true
	@echo ">> Python tests"
	@cd services/alfred && pytest -q || true
	@echo ">> Node tests"
	@cd portal && pnpm test || true

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=200

ps:
	$(COMPOSE) ps

smoke:
	@bash deploy/compose/scripts/smoke.sh

fmt:
	@cd services/control-plane && go fmt ./... || true
	@cd services/registry && go fmt ./... || true
	@cd services/audit && go fmt ./... || true
	@cd services/alfred && ruff format . || true

clean:
	@find . -type d -name node_modules -prune -exec rm -rf {} \; 2>/dev/null || true
	@find . -type d -name .next -prune -exec rm -rf {} \; 2>/dev/null || true
	@find . -type d -name __pycache__ -prune -exec rm -rf {} \; 2>/dev/null || true
