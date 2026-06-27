COMPOSE ?= docker compose

.PHONY: help build test lint run run-once docker-up docker-down docker-logs ollama-pull ollama-push db-migrate doctor clean fmt vet eval-quality eval-gate eval-target smoke-ui-docker-e2e

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
	@echo "docker-up        - Start all services using Docker Compose or COMPOSE='podman compose'"
	@echo "docker-down      - Stop all services using Docker Compose or COMPOSE='podman compose'"
	@echo "docker-logs      - Follow Compose logs using Docker or COMPOSE='podman compose'"
	@echo "ollama-pull      - Pull required LLM models to local Ollama"
	@echo "ollama-push      - Push persona Modelfiles to local Ollama"
	@echo "db-migrate       - Run database migrations (Postgres)"
	@echo "doctor           - Run environment health check"
	@echo "clean            - Remove built binaries and temporary files"
	@echo "clear            - Clear all database data, Docker volumes and builds"
	@echo "start-fresh      - Reset everything and run one pipeline pass"
	@echo "smoke-ui-docker-e2e - Run backend + UI Docker E2E smoke (requires both Compose stacks)"

build:
	go build -o build/server ./cmd/server
	go build -o build/alertd ./cmd/alertd

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
	go run cmd/server/main.go

run-alertd:
	go run cmd/alertd/main.go

run-once:
	go run cmd/server/main.go --run-once

eval-quality:
	go test -v ./internal/evalops/... -count=1

eval-gate:
	go test ./internal/evalops/... -count=1 -run "TestRunGate|TestLoadCases_FixtureDir|TestScoreLeadCase_FixtureCases|TestScoreAlertCase_FixtureCases"

eval-target:
	go run cmd/evaltarget/main.go

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f

ollama-pull:
	ollama pull mistral
	ollama pull llama3.1:8b
	ollama pull phi3:mini

ollama-push:
	go run cmd/server/main.go ollama push-models

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
	go run cmd/server/main.go --run-once
