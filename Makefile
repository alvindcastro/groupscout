.PHONY: help build test lint run run-once docker-up docker-down docker-logs compose-check secrets-edit secrets-render secrets-diff secrets-updatekeys secrets-scan ansible-deps ansible-apply ansible-check ansible-lint ollama-pull ollama-push db-migrate doctor clean fmt vet eval-quality eval-gate eval-target smoke-ui-docker-e2e

# Container runtime; `make docker-up COMPOSE="podman compose"` works too.
COMPOSE ?= docker compose
# Optional Compose profiles started with the core services (llm = Ollama,
# obs = Prometheus/Grafana/Loki/Alloy). `COMPOSE_PROFILES=` starts core only.
COMPOSE_PROFILES ?= llm,obs
export COMPOSE_PROFILES
# Git revision stamped into binaries and images; reported by /health.
GROUPSCOUT_VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
export GROUPSCOUT_VERSION
LDFLAGS := -X main.version=$(GROUPSCOUT_VERSION)

# Default target: help
help:
	@echo "GroupScout Development Makefile"
	@echo "-------------------------------"
	@echo "build            - Build the server and alertd binaries"
	@echo "test             - Run all Go tests"
	@echo "fmt              - Format all Go files"
	@echo "vet              - Run go vet"
	@echo "eval-quality     - Run Go eval harness against golden fixtures (no live APIs)"
	@echo "eval-gate        - Run release gate: exit non-zero on critical failures"
	@echo "eval-target      - Start the local eval target HTTP server on :18080 for Promptfoo"
	@echo "lint             - Run golangci-lint (if installed)"
	@echo "run              - Run the lead generation server"
	@echo "run-alertd       - Run the alertd service"
	@echo "run-once         - Run the lead generation pipeline once and exit"
	@echo "docker-up        - Build and start services (COMPOSE_PROFILES=$(COMPOSE_PROFILES))"
	@echo "docker-down      - Stop all services using Docker Compose"
	@echo "docker-logs      - Follow Docker Compose logs"
	@echo "compose-check    - Render dev (every profile) and prod Compose configs"
	@echo "secrets-edit     - Edit deploy/secrets/\$$SECRETS_ENV.sops.env (default prod) with sops"
	@echo "secrets-render   - Decrypt it to .env (FORCE=1 replaces a .env that differs)"
	@echo "secrets-diff     - Name the keys that differ between .env and the encrypted file"
	@echo "secrets-updatekeys - Re-encrypt secrets after changing recipients in .sops.yaml"
	@echo "secrets-scan     - gitleaks over git history plus tracked-secret-file checks"
	@echo "ansible-deps     - Install the pinned Ansible collections (deploy/ansible/requirements.yml)"
	@echo "ansible-apply    - Configure the host and start the stack (deploy/ansible/site.yml)"
	@echo "ansible-check    - Drift check: what ansible-apply would change (--check --diff)"
	@echo "ansible-lint     - Lint the playbook and roles"
	@echo "ollama-pull      - Pull required LLM models to local Ollama"
	@echo "ollama-push      - Push persona Modelfiles to local Ollama"
	@echo "db-migrate       - Run database migrations (Postgres)"
	@echo "doctor           - Run environment health check"
	@echo "clean            - Remove built binaries and temporary files"
	@echo "clear            - Clear all database data, Docker volumes and builds"
	@echo "start-fresh      - Reset everything and run one pipeline pass"
	@echo "smoke-ui-docker-e2e - Run backend + UI Docker E2E smoke (requires both Compose stacks)"

build:
	go build -ldflags "$(LDFLAGS)" -o build/server ./cmd/server
	go build -ldflags "$(LDFLAGS)" -o build/alertd ./cmd/alertd

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it from https://golangci-lint.run/"; \
		exit 1; \
	fi

run:
	go run ./cmd/server

run-alertd:
	go run ./cmd/alertd

run-once:
	go run ./cmd/server --run-once

eval-quality:
	go test -v ./internal/evalops/... -count=1

eval-gate:
	go test ./internal/evalops/... -count=1 -run "TestRunGate|TestLoadCases_FixtureDir|TestScoreLeadCase_FixtureCases|TestScoreAlertCase_FixtureCases"

eval-target:
	go run cmd/evaltarget/main.go

docker-up:
	$(COMPOSE) up -d --build

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f

# compose-check: render every dev profile set and the prod overlay (with
# placeholder secrets) without starting anything.
compose-check:
	@for p in "" llm obs llm,obs; do \
		COMPOSE_PROFILES=$$p $(COMPOSE) config -q || exit 1; \
	done
	@GROUPSCOUT_IMAGE=ghcr.io/example/groupscout GROUPSCOUT_IMAGE_TAG=check \
		POSTGRES_PASSWORD=check API_TOKEN=check N8N_ENCRYPTION_KEY=check GRAFANA_ADMIN_PASSWORD=check \
		$(COMPOSE) -f docker-compose.yml -f deploy/compose/compose.prod.yml config -q
	@echo "compose-check: dev and prod Compose configs render"

# Secrets as code (SOPS + age): see deploy/scripts/secrets.sh and
# groupscout-site backend/docs/guides/SECRETS.md.
secrets-edit secrets-render secrets-diff secrets-updatekeys secrets-scan:
	@sh deploy/scripts/secrets.sh $(@:secrets-%=%)

# Host configuration (IaC Phase 5): deploy/ansible. ANSIBLE_CONFIG is set
# explicitly because Ansible ignores an ansible.cfg in a world-writable
# directory, which every path under /mnt/c is in WSL.
ANSIBLE_ENV := cd deploy/ansible && ANSIBLE_CONFIG=ansible.cfg

ansible-deps:
	$(ANSIBLE_ENV) ansible-galaxy collection install -r requirements.yml

ansible-apply:
	$(ANSIBLE_ENV) ansible-playbook site.yml

ansible-check:
	$(ANSIBLE_ENV) ansible-playbook site.yml --check --diff

ansible-lint:
	$(ANSIBLE_ENV) ansible-lint --offline site.yml

ollama-pull:
	ollama pull mistral
	ollama pull llama3.1:8b
	ollama pull phi3:mini

ollama-push:
	go run ./cmd/server ollama push-models

db-migrate:
	@echo "Ensure DATABASE_URL is set in your environment."
	# Add your migration command here if using a specific tool, 
	# otherwise it's handled by the app on startup.

doctor:
	@chmod +x scripts/doctor.sh
	@./scripts/doctor.sh

clean:
	rm -rf build/
	rm -f coverage.out coverage.html

# clear: Removes all data, stops containers, and cleans build artifacts
clear:
	@echo "Clearing all data..."
	$(COMPOSE) down -v
	rm -f groupscout.db
	rm -rf build/
	@echo "Data cleared."

smoke-ui-docker-e2e:
	@chmod +x scripts/smoke-ui-docker-e2e.sh
	@./scripts/smoke-ui-docker-e2e.sh

# start-fresh: Clears everything, starts services, and runs one pipeline pass
start-fresh: clear docker-up
	@echo "Waiting for services to be ready..."
	@sleep 15
	go run ./cmd/server --run-once
