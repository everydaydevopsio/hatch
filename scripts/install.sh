#!/usr/bin/env bash
set -euo pipefail
if ! command -v docker >/dev/null 2>&1; then echo "Docker is required. Install Docker Engine and the Compose plugin first." >&2; exit 1; fi
if ! docker compose version >/dev/null 2>&1; then echo "Docker Compose v2 is required." >&2; exit 1; fi
if [[ ! -f .env ]]; then
  cp .env.example .env
  if command -v openssl >/dev/null 2>&1; then
    PASSWORD="$(openssl rand -hex 24)"
    sed -i "s/replace-with-a-long-random-password/$PASSWORD/" .env
    echo "Created .env with a random RDP password. Store it securely."
  else
    echo "Created .env. Set RDP_PASSWORD before starting." >&2
  fi
fi
docker compose build
echo
echo "Build complete."
echo "Start with: docker compose up -d"
echo "Check with: docker compose ps"
