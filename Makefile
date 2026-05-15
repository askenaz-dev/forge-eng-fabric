# Forge Engineering Fabric — root Makefile
# Targets are designed to work on Windows (pwsh) and Unix shells where reasonable.
# On Windows, run via `make` from Git Bash, WSL, or use `pwsh` equivalents.

SHELL := /bin/sh
COMPOSE := docker compose -f deploy/compose/docker-compose.yaml

.PHONY: help bootstrap lint test codegen up down logs ps smoke clean fmt sizing-check helm-lint verify-runtime demo-intent-to-deploy demo-intent-to-infrastructure retention-policy-check portal-rebrand-e2e audit-no-mocks dev-up seed-portal portal-lint package-skill

help:
	@echo "Forge — make targets:"
	@echo "  bootstrap   Install local toolchain prerequisites (info only)"
	@echo "  lint        Run linters across Go / Python / Node"
	@echo "  test        Run tests across Go / Python / Node"
	@echo "  codegen     Generate OpenAPI clients into contracts/generated"
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

codegen:
	@bash contracts/openapi/codegen.sh

# Package a skill from a YAML spec into an Agent Skills bundle.
# Usage: make package-skill SPEC=path/to/skill.yaml OUT=path/to/out.tar.zst
package-skill:
	@cd pkg/skill-packager && go run ./cmd/package-skill -spec="$(SPEC)" -out="$(OUT)"

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

sizing-check:
	@python scripts/check-sizing.py

helm-lint:
	@bash scripts/helm-lint.sh

verify-runtime:
	@python scripts/verify_runtime.py $(if $(WORKSPACE),--workspace $(WORKSPACE)) $(if $(RUNTIME),--runtime $(RUNTIME))

demo-intent-to-deploy:
	@python scripts/demo_intent_to_deploy.py

# sdlc-end-to-end: exercises forge.reference.intent-to-infrastructure@1 end-to-end
# against the local stack. Requires: make up, a registered Minikube runtime, and
# the feature flags forge.workflow.intent_to_infrastructure.enabled=true (per-tenant).
# Output: build/demo-intent-to-infrastructure/<timestamp>.json
demo-intent-to-infrastructure:
	@mkdir -p build/demo-intent-to-infrastructure
	@python scripts/demo_intent_to_infrastructure.py

# Optimize Alfred SVG marks, copy to portal/public, and regenerate the Alfred
# section of the standalone brand notebook. svgo is optional — if missing the
# raw SVGs are copied through unchanged.
design-export:
	@python scripts/design_export_alfred.py
	@echo ">> design-export: alfred marks regenerated"

# Verify design/alfred-identity/ has not drifted from the notebook section.
design-export-check:
	@python scripts/design_export_alfred.py --check

retention-policy-check:
	@python scripts/check-retention-policy.py

# Bring up the dev compose stack used by the Portal e2e suite.
dev-up: up
	@echo ">> dev-up: docker stack started"

# Seed deterministic data the Portal e2e suite expects (workspaces, agents,
# runs, approvals, audit events, KPIs). Idempotent.
seed-portal:
	@bash deploy/compose/scripts/seed-portal.sh

# Forge Portal rebrand e2e: brings up the dev stack, seeds, then runs
# Playwright against the live cluster. No MSW, no fixtures.
portal-rebrand-e2e: dev-up seed-portal
	@cd portal && pnpm install --frozen-lockfile=false
	@cd portal && pnpm exec playwright install --with-deps chromium
	@cd portal && PORTAL_REBRAND=1 pnpm exec playwright test

# Audit script that enforces the "real data only" policy in portal/src/.
audit-no-mocks:
	@bash scripts/audit-no-mocks.sh

# Convenience target that runs ESLint + Stylelint + the no-mocks audit.
portal-lint:
	@cd portal && pnpm lint
	@cd portal && pnpm stylelint
	@bash scripts/audit-no-mocks.sh
