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
	@# Database first: migrations and the app login role (owner credentials, ops container) before the app starts,
	@# which connects as that role (ISS-039).
	$(COMPOSE) up -d --wait db redis
	$(MAKE) migrate
	$(COMPOSE) up -d --wait
	$(MAKE) --no-print-directory version-check

version-check: .env ## Verify the running /api/v1/version == git describe of HEAD (internal listener)
	@# Retries up to 30 s: right after the app container is recreated Caddy may still route to the old one (ISS-025).
	@set -a; source .env; set +a; want='$(VERSION)'; base="http://$${CADDY_HTTP_BIND:-127.0.0.1:8080}"; got=""; \
	for i in $$(seq 1 30); do \
	  got="$$(curl -fsS --max-time 3 "$$base/api/v1/version" 2>/dev/null | jq -r .version 2>/dev/null)"; \
	  [[ "$$got" == "$$want" ]] && break; sleep 1; \
	done; \
	if [[ "$$got" != "$$want" ]]; then echo "version-check: running '$$got' != git describe $$want after 30 s (run: make up)" >&2; exit 1; fi; \
	[[ $$i -gt 1 ]] && echo "version-check: OK $$got (after $$i attempts)" || echo "version-check: OK $$got"

preflight: .env ## Validate .env before starting (owner-only permissions, domain/ACME, secrets format)
	@./scripts/env-perm-check.sh
	@set -a; source .env; set +a; \
	if [[ -n "$${DOMAIN:-}" && "$$DOMAIN" != localhost && -z "$${ACME_EMAIL:-}" ]]; then \
	  echo "preflight: DOMAIN=$$DOMAIN requires ACME_EMAIL in .env" >&2; exit 1; fi; \
	if (( $${#DB_APP_PASSWORD} < 20 )); then \
	  echo "preflight: DB_APP_PASSWORD must be at least 20 characters (openssl rand -hex 24), see .env.example" >&2; exit 1; fi; \
	if [[ ! "$$(printf '%s' "$${MFA_ENC_KEY:-}" | base64 -d 2>/dev/null | wc -c)" == 32 ]]; then \
	  echo "preflight: MFA_ENC_KEY must be 32 random bytes in base64 (openssl rand -base64 32), see .env.example" >&2; exit 1; fi; \
	if [[ ! '$(VERSION)' =~ ^[A-Za-z0-9._-]+$$ ]]; then echo "preflight: invalid VERSION '$(VERSION)'" >&2; exit 1; fi

CADDY_VALIDATE := docker run --rm -v "$$PWD/deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro"
caddy-validate: ## Validate the Caddyfile for production-like and empty (dev/CI) settings
	$(CADDY_VALIDATE) -e DOMAIN=sentra.example.go.id -e ACME_EMAIL=admin@example.go.id caddy:2.11.7-alpine caddy validate --config /etc/caddy/Caddyfile
	$(CADDY_VALIDATE) -e DOMAIN=localhost -e ACME_EMAIL=admin@localhost caddy:2.11.7-alpine caddy validate --config /etc/caddy/Caddyfile

web-check: ## Web UI checks in Docker (no Node on host): lint, typecheck, i18n:check, unit tests
	docker build --target check --output type=cacheonly web

down: ## Stop containers (volumes are kept)
	$(COMPOSE) down

logs: ## Follow container logs (SERVICE=app to filter)
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

ps: ## Show container status
	$(COMPOSE) ps

build: ## Build backend binary to bin/sentra
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/sentra ./cmd/sentra

## ---------- Quality ----------
.PHONY: web-check test test-integration test-api test-ui test-e2e lint vulncheck fail2ban-test fail2ban-web-check fail2ban-web-check-test fail2ban-web fail2ban-web-remove fail2ban-ban-test boot-menu post-reboot-check recovery-drill gitleaks openapi-lint sqlc fmt
test: ## Run unit tests with race detector and coverage
	CGO_ENABLED=1 go test -race -count=1 -coverprofile=coverage.out ./...

test-integration: ## Run integration tests against running DB/Redis (requires `make up`)
	set -a; source .env; set +a; \
	SENTRA_IT_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:$${POSTGRES_HOST_PORT:-5432}/$$POSTGRES_DB?sslmode=disable" \
	SENTRA_IT_REDIS_ADDR="127.0.0.1:$${REDIS_HOST_PORT:-6379}" SENTRA_IT_REDIS_PASSWORD="$$REDIS_PASSWORD" \
	go test -count=1 -tags=integration ./...

test-api: ## Run API smoke tests against the running stack (BASE_URL overridable)
	BASE_URL=$${BASE_URL:-http://127.0.0.1:8080} ./test/api/smoke.sh

test-ui: ## UI edge tests (basic_auth, SPA, headers) against the public site; credentials: see test/api/ui.sh
	./test/api/ui.sh

test-e2e: ## Browser smoke test (Playwright in Docker) against the public site; credentials: see test/e2e/README.md
	./test/e2e/run.sh

lint: ## Run golangci-lint
	golangci-lint run ./...

fail2ban-test: ## Test the fail2ban filter against synthetic Caddy log samples (host fail2ban-regex or Docker)
	./scripts/fail2ban-regex-test.sh

fail2ban-web-check: ## Compare the installed jail sentra-web with the repository (sha256, no sudo; used by make release)
	./scripts/fail2ban-web-check.sh

fail2ban-web-check-test: ## Test the drift check and release.sh fail2ban gate in a temporary directory (no sudo)
	./scripts/fail2ban-web-check-test.sh

fail2ban-web: ## Install/update the fail2ban jail sentra-web for failed app sign-ins/MFA codes, ADMIN_IPS exempt (idempotent; sudo)
	sudo ./scripts/fail2ban-web-install.sh

fail2ban-web-remove: ## Remove the sentra-web jail, filter and systemd drop-in (sudo)
	sudo ./scripts/fail2ban-web-install.sh --remove

fail2ban-ban-test: ## Real ban/unban test from a throwaway container network against this host (sudo)
	sudo ./scripts/fail2ban-ban-test.sh

boot-menu: ## Host boot prep: GRUB menu 10 s on the serial console, boot services enabled (idempotent; sudo)
	sudo ./scripts/host-boot-setup.sh

post-reboot-check: ## After a reboot: kernel, uptime, services, containers, fail2ban/DOCKER-USER, smoke, edge, ban test (sudo)
	sudo ./scripts/post-reboot-check.sh

recovery-drill: ## Build and start the whole stack from a clean GitHub clone (separate project/ports), then clean up
	./scripts/recovery-drill.sh

vulncheck: ## Scan Go dependencies for known, reachable vulnerabilities (same version as CI)
	go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
	govulncheck ./...

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
migrate: ## Apply pending migrations and create/rotate the app login role (ops container, owner credentials)
	$(COMPOSE) run --rm --no-deps ops migrate up
	$(COMPOSE) run --rm --no-deps ops db sync-app-role

migrate-down: ## Roll back the last migration (STEPS=n for more)
	$(COMPOSE) run --rm --no-deps ops migrate down $${STEPS:-1}

migrate-version: ## Show current migration version
	$(COMPOSE) run --rm --no-deps ops migrate version

migrate-goto: ## Migrate to a specific version (V=n)
	$(COMPOSE) run --rm --no-deps ops migrate goto $(V)

## ---------- Ops & DR ----------
.PHONY: backup restore rollback release harden backup-timer audit-verify audit-prune
backup: ## Backup DB + important volumes to $$BACKUP_DIR
	./ops/backup.sh

audit-verify: ## Verify the audit hash chain against the anchors in all backups (UNTIL=<backup dir> after a restore)
	./ops/audit-verify.sh

audit-prune: ## Archive audit events older than AUDIT_RETENTION_DAYS to $$BACKUP_DIR/audit-archive, then prune (checkpoint)
	./ops/audit-prune.sh

restore: ## Restore from a backup directory (FROM=/var/backups/sentra/<dir>; ARGS="--target-db x --yes")
	./ops/restore.sh $(FROM) $(ARGS)

rollback: ## Roll back to a git tag (TAG=vX.Y.Z): backup -> checkout -> migrate -> up
	./ops/rollback.sh $(TAG)

release: ## In the main tree: merge feat/V-*, tag, push, redeploy; stops if the fail2ban jail drifted (V=vX.Y.Z T="Title")
	./scripts/release.sh $(V) "$(T)"

backup-timer: ## Install the systemd daily backup timer (02:00 Asia/Jakarta); needs sudo
	sudo ./ops/install-backup-timer.sh

harden: ## Apply server hardening (ufw, fail2ban, sysctl, unattended-upgrades); needs sudo
	sudo ./ops/harden/all.sh
