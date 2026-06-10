#!/usr/bin/env bash
set -euo pipefail

MODEL="${MODEL:-phi3}"
SOURCE="${1:-}"
OUTPUT="${2:-a.out}"

if [[ -z "$SOURCE" ]]; then
  echo "Usage: $0 <source-file> [output-binary]"
  echo "       MODEL=codellama $0 main.c hello"
  exit 1
fi

# Build sloppiler if needed
if [[ ! -f ./sloppiler ]]; then
  echo "[setup] building sloppiler..."
  go build -o sloppiler .
fi

# Start ollama in background if not already running
if ! curl -sf http://localhost:11434 > /dev/null 2>&1; then
  echo "[setup] starting ollama..."
  ollama serve &
  OLLAMA_PID=$!
  # Wait for ollama to be ready
  for i in $(seq 1 20); do
    sleep 0.5
    if curl -sf http://localhost:11434 > /dev/null 2>&1; then break; fi
    if [[ $i -eq 20 ]]; then echo "[error] ollama did not start in time"; exit 1; fi
  done
fi

# Pull model if not present
if ! ollama list | grep -q "^${MODEL}"; then
  echo "[setup] pulling model ${MODEL}..."
  ollama pull "$MODEL"
fi

echo "[run] sloppiling $SOURCE -> $OUTPUT with model $MODEL"
./sloppiler -model "$MODEL" -o "$OUTPUT" "$SOURCE"
