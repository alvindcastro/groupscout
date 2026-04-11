#!/bin/bash

# Simple script to test Ollama integration

# 1. Check if Ollama is running
if ! curl -s http://localhost:11434/api/tags > /dev/null; then
  echo "❌ Error: Ollama is not running on http://localhost:11434"
  echo "Make sure Ollama is installed and running."
  exit 1
else
  echo "✅ Ollama is running."
fi

# 2. Check for required models
REQUIRED_MODELS=("mistral" "llama3.1:8b" "phi3:mini")
AVAILABLE_MODELS=$(curl -s http://localhost:11434/api/tags | grep -o '"name":"[^"]*"' | cut -d'"' -f4)

for model in "${REQUIRED_MODELS[@]}"; do
  if echo "$AVAILABLE_MODELS" | grep -q "$model"; then
    echo "✅ Model '$model' is available."
  else
    echo "❌ Model '$model' is NOT available. Run 'ollama pull $model'."
  fi
done

# 3. Test simple inference (mistral)
echo "Testing simple inference with 'mistral'..."
RESPONSE=$(curl -s -X POST http://localhost:11434/api/generate -d '{
  "model": "mistral",
  "prompt": "Say hello in one word",
  "stream": false
}')

if echo "$RESPONSE" | grep -q '"response":"'; then
  echo "✅ Inference test successful."
  echo "Response: $(echo "$RESPONSE" | grep -o '"response":"[^"]*"' | cut -d'"' -f4)"
else
  echo "❌ Inference test failed."
  echo "Raw response: $RESPONSE"
fi

# 4. Check GroupScout persona models
echo "Checking GroupScout persona models..."
PERSONA_MODELS=("permit_extractor" "lead_scorer" "disruption_alert")

for persona in "${PERSONA_MODELS[@]}"; do
  if echo "$AVAILABLE_MODELS" | grep -q "$persona"; then
    echo "✅ Persona model '$persona' is available."
  else
    echo "ℹ️ Persona model '$persona' is NOT available. Run 'go run cmd/server/main.go ollama push-models'."
  fi
done
