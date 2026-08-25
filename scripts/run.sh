#!/usr/bin/env bash
set -euo pipefail
if [[ ! -f .env ]]; then echo ".env is missing. Run ./scripts/install.sh first or copy .env.example to .env." >&2; exit 1; fi
docker compose up -d
echo "Hatch started."
echo "Use the server's private/Tailscale IP in your RDP client on port 3389."
