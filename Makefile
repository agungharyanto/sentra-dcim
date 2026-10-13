# Sentra DCIM — task runner. Run `make help` for the list of targets.
SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

export PATH := $(PATH):/usr/local/go/bin:$(HOME)/go/bin

# Always derived from git (`:=`, not `?=`): an exported VERSION in the shell — e.g. after `. /etc/os-release`,
# which sets VERSION="24.04.5 LTS (Noble Numbat)" — must never leak into the build (ISS-022).
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.Version=$(VERSION) \
	-X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.Commit=$(COMMIT) \
	-X github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo.BuildDate=$(BUILD_DATE)

COMPOSE := VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE) docker compose

.PHONY: help
help: ## Show this help
	@awk 'BEGIN{FS=":.*## "} /^[a-zA-Z_-]+:.*## /{printf "  \033[36m%-18s\033[0m %s\n",$$1,$$2}' $(MAKEFILE_LIST)

## ---------- Setup ----------
.PHONY: hooks
hooks: ## Install git pre-commit hook (gitleaks)
	git config core.hooksPath scripts/hooks
	@echo "pre-commit hook installed (core.hooksPath=scripts/hooks)"

.env:
	@echo ".env missing: cp .env.example .env && chmod 600 .env, then fill in secrets" >&2; exit 1

## ---------- Containers ----------
.PHONY: up down logs ps build preflight caddy-validate version-check
up: .env preflight ## Rebuild images for the current `git describe`, start all containers, migrate, verify version
	$(COMPOSE) build app caddy
	$(COMPOSE) up -d --wait
	$(MAKE) migrate
	$(MAKE) --no-print-directory version-check

version-check: .env ## Verify the running /api/v1/version == git describe of HEAD (internal listener)
	@set -a; source .env; set +a; want='$(VERSION)'; base="http://$${CADDY_HTTP_BIND:-127.0.0.1:8080}"; \
	got="$$(curl -fsS "$$base/api/v1/version" | jq -r .version)"; \
	if [[ "$$got" != "$$want" ]]; then echo "version-check: running $$got != git describe $$want (run: make up)" >&2; exit 1; fi; \
	echo "version-check: OK $$got"

preflight: .env ## Validate .env before starting (domain/ACME settings)
	@set -a; source .env; set +a; \
	if [[ -n "$${DOMAIN:-}" && "$$DOMAIN" != localhost && -z "$${ACME_EMAIL:-}" ]]; then \
	  echo "preflight: DOMAIN=$$DOMAIN requires ACME_EMAIL in .env" >&2; exit 1; fi; \
	if [[ ! '$(VERSION)' =~ ^[A-Za-z0-9._-]+$$ ]]; then echo "preflight: invalid VERSION '$(VERSION)'" >&2; exit 1; fi

caddy-validate: ## Validate the Caddyfile for production-like and empty (dev/CI) settings
	docker build -q -t sentra-caddy:validate deploy/caddy >/dev/null
	docker run --rm -e DOMAIN=sentra.example.go.id -e ACME_EMAIL=admin@example.go.id --entrypoint caddy sentra-caddy:validate validate --config /etc/caddy/Caddyfile
	docker run --rm -e DOMAIN=localhost -e ACME_EMAIL=admin@localhost --entrypoint caddy sentra-caddy:validate validate --config /etc/caddy/Caddyfile

down: ## Stop containers (volumes are kept)
	$(COMPOSE) down

logs: ## Follow container logs (SERVICE=app to filter)
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

ps: ## Show container status
	$(COMPOSE) ps

build: ## Build backend binary to bin/sentra
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/sentra ./cmd/sentra

## ---------- Quality ----------
.PHONY: test test-integration test-api lint gitleaks openapi-lint sqlc fmt
test: ## Run unit tests with race detector and coverage
	CGO_ENABLED=1 go test -race -count=1 -coverprofile=coverage.out ./...

test-integration: ## Run integration tests against running DB/Redis (requires `make up`)
	set -a; source .env; set +a; \
	SENTRA_IT_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:$${POSTGRES_HOST_PORT:-5432}/$$POSTGRES_DB?sslmode=disable" \
	SENTRA_IT_REDIS_ADDR="127.0.0.1:$${REDIS_HOST_PORT:-6379}" SENTRA_IT_REDIS_PASSWORD="$$REDIS_PASSWORD" \
	go test -count=1 -tags=integration ./...

test-api: ## Run API smoke tests against the running stack (BASE_URL overridable)
	BASE_URL=$${BASE_URL:-http://127.0.0.1:8080} ./test/api/smoke.sh

lint: ## Run golangci-lint
	golangci-lint run ./...

gitleaks: ## Scan full git history and all non-ignored working tree files for secrets
	gitleaks git --redact --no-banner --config .gitleaks.toml .
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	git ls-files -co --exclude-standard -z | xargs -0 cp --parents -t "$$tmp"; \
	gitleaks dir --redact --no-banner --config .gitleaks.toml "$$tmp"

openapi-lint: ## Validate api/openapi.yaml with Redocly CLI (docker)
	docker run --rm -v "$$PWD/api:/spec:ro" redocly/cli:2.62.1 lint /spec/openapi.yaml

sqlc: ## Regenerate sqlc code
	sqlc generate

fmt: ## Format Go code
	gofmt -s -w cmd internal

## ---------- Database ----------
.PHONY: migrate migrate-down migrate-version migrate-goto
migrate: ## Apply all pending migrations
	$(COMPOSE) exec -T app /sentra migrate up

migrate-down: ## Roll back the last migration (STEPS=n for more)
	$(COMPOSE) exec -T app /sentra migrate down $${STEPS:-1}

migrate-version: ## Show current migration version
	$(COMPOSE) exec -T app /sentra migrate version

migrate-goto: ## Migrate to a specific version (V=n)
	$(COMPOSE) exec -T app /sentra migrate goto $(V)

## ---------- Ops & DR ----------
.PHONY: backup restore rollback release harden backup-timer
backup: ## Backup DB + important volumes to $$BACKUP_DIR
	./ops/backup.sh

restore: ## Restore from a backup directory (FROM=/var/backups/sentra/<dir>; ARGS="--target-db x --yes")
	./ops/restore.sh $(FROM) $(ARGS)

rollback: ## Roll back to a git tag (TAG=vX.Y.Z): backup -> checkout -> migrate -> up
	./ops/rollback.sh $(TAG)

release: ## Merge current feature branch to main, tag and push (V=vX.Y.Z T="Title")
	./scripts/release.sh $(V) "$(T)"

backup-timer: ## Install the systemd daily backup timer (02:00 Asia/Jakarta); needs sudo
	sudo ./ops/install-backup-timer.sh

harden: ## Apply server hardening (ufw, fail2ban, sysctl, unattended-upgrades); needs sudo
	sudo ./ops/harden/all.sh
