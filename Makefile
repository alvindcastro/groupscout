.PHONY: help build test lint run run-once docker-up docker-down docker-logs ollama-pull ollama-push db-migrate doctor clean fmt vet

# Default target: help
help:
	@echo "GroupScout Development Makefile"
	@echo "-------------------------------"
	@echo "build            - Build the server and alertd binaries"
	@echo "test             - Run all Go tests"
	@echo "fmt              - Format all Go files"
	@echo "vet              - Run go vet"
	@echo "lint             - Run golangci-lint (if installed)"
	@echo "run              - Run the lead generation server"
	@echo "run-alertd       - Run the alertd service"
	@echo "run-once         - Run the lead generation pipeline once and exit"
	@echo "docker-up        - Start all services using Docker Compose"
	@echo "docker-down      - Stop all services using Docker Compose"
	@echo "docker-logs      - Follow Docker Compose logs"
	@echo "ollama-pull      - Pull required LLM models to local Ollama"
	@echo "ollama-push      - Push persona Modelfiles to local Ollama"
	@echo "db-migrate       - Run database migrations (Postgres)"
	@echo "doctor           - Run environment health check"
	@echo "clean            - Remove built binaries and temporary files"

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

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

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
