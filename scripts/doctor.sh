#!/bin/bash

# GroupScout Doctor — Environment Health Check
# Checks if the development environment is correctly configured.

BOLD='\033[1m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${BOLD}GroupScout Environment Doctor${NC}"
echo "-------------------------------"

# 1. Check Go
if command -v go > /dev/null; then
  GO_VERSION=$(go version | awk '{print $3}')
  echo -e "✅ Go is installed ($GO_VERSION)"
else
  echo -e "❌ Go is NOT installed. Install it from https://golang.org/"
fi

# 2. Check Docker
if command -v docker > /dev/null; then
  DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
  if [ $? -eq 0 ]; then
    echo -e "✅ Docker is installed and running ($DOCKER_VERSION)"
  else
    echo -e "⚠️ Docker is installed but NOT running."
  fi
else
  echo -e "❌ Docker is NOT installed."
fi

# 3. Check curl
if command -v curl > /dev/null; then
  echo -e "✅ curl is installed."
else
  echo -e "❌ curl is NOT installed."
fi

# 4. Check .env file
if [ -f .env ]; then
  echo -e "✅ .env file exists."
else
  echo -e "⚠️ .env file is missing. Copy .env.example to .env and configure it."
fi

# 5. Check Ollama
echo -e "\n${BOLD}Ollama Integration${NC}"
if curl -s http://localhost:11434/api/tags > /dev/null; then
  echo -e "✅ Ollama is running on http://localhost:11434"
  
  # Check models
  AVAILABLE_MODELS=$(curl -s http://localhost:11434/api/tags | grep -o '"name":"[^"]*"' | cut -d'"' -f4)
  REQUIRED_MODELS=("mistral" "phi3:mini")
  
  for model in "${REQUIRED_MODELS[@]}"; do
    if echo "$AVAILABLE_MODELS" | grep -q "$model"; then
      echo -e "  ✅ Model '$model' is available."
    else
      echo -e "  ❌ Model '$model' is NOT available. Run 'ollama pull $model'."
    fi
  done
  
  # Check persona models
  PERSONA_MODELS=("permit_extractor" "lead_scorer")
  for persona in "${PERSONA_MODELS[@]}"; do
    if echo "$AVAILABLE_MODELS" | grep -q "$persona"; then
      echo -e "  ✅ Persona model '$persona' is available."
    else
      echo -e "  ℹ️ Persona model '$persona' is NOT available. Run 'make ollama-push'."
    fi
  done
else
  echo -e "❌ Ollama is NOT running on http://localhost:11434"
  echo "   If you're using Docker, run 'docker compose up -d ollama'."
fi

# 6. Check Database
echo -e "\n${BOLD}Database Configuration${NC}"
if [ -f .env ]; then
  DB_URL=$(grep "^DATABASE_URL=" .env | cut -d'=' -f2)
  if [[ $DB_URL == postgres://* ]]; then
    echo -e "ℹ️ Configured for Postgres: $DB_URL"
    # Could add a pg_isready check here if needed
  else
    echo -e "ℹ️ Configured for SQLite: $DB_URL"
    if [ -f "$DB_URL" ]; then
      echo -e "  ✅ SQLite database file exists."
    else
      echo -e "  ℹ️ SQLite database file will be created on first run."
    fi
  fi
else
  echo -e "⚠️ Unable to check database config (no .env)."
fi

echo "-------------------------------"
echo "Done."
