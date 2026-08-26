#!/usr/bin/env bash
set -euo pipefail

IMAGE="${HATCH_E2E_IMAGE:-hatch:e2e}"
PREFIX="${HATCH_E2E_NAME:-hatch-e2e}"
HTTPS_PORT="${HATCH_E2E_HTTPS_PORT:-18443}"
URL="${HATCH_E2E_URL:-https://www.google.com}"
RDP_USER="${HATCH_E2E_RDP_USER:-oauth}"
TMP_DIR="$(mktemp -d)"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: $1 is required." >&2
    exit 2
  fi
}

cleanup() {
  docker rm -f "${PREFIX}-hatch" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_command docker
require_command curl
require_command nc
require_command node
require_command npm

cleanup
mkdir -p "$TMP_DIR"

docker build -t "$IMAGE" .

docker run -d \
  --name "${PREFIX}-hatch" \
  -p "127.0.0.1:${HTTPS_PORT}:443" \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER="$RDP_USER" \
  -e HATCH_START_URL="$URL" \
  "$IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if [ "$(docker inspect "${PREFIX}-hatch" --format '{{.State.Health.Status}}' 2>/dev/null || true)" = "healthy" ]; then
    break
  fi
  sleep 1
done

if [ "$(docker inspect "${PREFIX}-hatch" --format '{{.State.Health.Status}}')" != "healthy" ]; then
  docker logs "${PREFIX}-hatch" >&2 || true
  echo "ERROR: Hatch container did not become healthy." >&2
  exit 1
fi

for _ in $(seq 1 30); do
  if nc -z 127.0.0.1 "$HTTPS_PORT" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! nc -z 127.0.0.1 "$HTTPS_PORT" >/dev/null 2>&1; then
  docker logs "${PREFIX}-hatch" >&2 || true
  echo "ERROR: Hatch did not open HTTPS port $HTTPS_PORT." >&2
  exit 1
fi

if ! curl -kfsS "https://127.0.0.1:${HTTPS_PORT}/guacamole/" >/dev/null; then
  docker logs "${PREFIX}-hatch" >&2 || true
  echo "ERROR: Guacamole did not respond through HTTPS." >&2
  exit 1
fi

GUAC_USER="$(docker logs "${PREFIX}-hatch" 2>&1 | sed -n 's/^Guacamole user: //p' | tail -n 1)"
GUAC_PASSWORD="$(docker logs "${PREFIX}-hatch" 2>&1 | sed -n 's/^Guacamole password: //p' | tail -n 1)"
if [ -z "$GUAC_USER" ] || [ -z "$GUAC_PASSWORD" ]; then
  docker logs "${PREFIX}-hatch" >&2 || true
  echo "ERROR: Could not extract generated Guacamole credentials from Hatch logs." >&2
  exit 1
fi

cat > "$TMP_DIR/guac-smoke.mjs" <<'JS'
import { chromium } from 'playwright';

const baseUrl = process.env.GUAC_URL;
const username = process.env.GUAC_USER;
const password = process.env.GUAC_PASSWORD;

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({
  ignoreHTTPSErrors: true,
  viewport: { width: 1280, height: 900 },
});
const page = await context.newPage();

await page.goto(`${baseUrl}/guacamole/`, { waitUntil: 'domcontentloaded' });
await page.getByLabel(/username/i).fill(username);
await page.getByLabel(/password/i).fill(password);
await page.getByRole('button', { name: /login/i }).click();

const clientId = Buffer.from('Hatch\0c\0default').toString('base64').replace(/=/g, '');
await page.goto(`${baseUrl}/guacamole/#/client/${clientId}`, { waitUntil: 'domcontentloaded' });
await page.locator('canvas').first().waitFor({ timeout: 60000 });
await page.waitForTimeout(5000);

await browser.close();
JS

GUAC_URL="https://127.0.0.1:${HTTPS_PORT}" \
GUAC_USER="$GUAC_USER" \
GUAC_PASSWORD="$GUAC_PASSWORD" \
  sh -c 'cd "$1" && npm init -y >/dev/null && npm install playwright@1.57.0 >/dev/null && npx playwright install chromium >/dev/null && node guac-smoke.mjs' sh "$TMP_DIR" || {
    echo "ERROR: Playwright could not complete the Guacamole desktop flow." >&2
    echo "---- Hatch listeners ----" >&2
    docker exec "${PREFIX}-hatch" sh -lc "ss -ltnp | grep -E ':(443|8080|4822|3389)' || true" >&2 || true
    echo "---- Hatch logs ----" >&2
    docker logs "${PREFIX}-hatch" >&2 || true
    exit 1
  }

for _ in $(seq 1 45); do
  if docker exec "${PREFIX}-hatch" sh -lc "pgrep -af 'chromium.*${URL}' >/dev/null"; then
    echo "Guacamole E2E succeeded: HTTPS login reached Hatch RDP and Chromium opened $URL"
    exit 0
  fi
  sleep 1
done

echo "ERROR: Chromium process for $URL was not observed in Hatch." >&2
echo "---- Hatch listeners ----" >&2
docker exec "${PREFIX}-hatch" sh -lc "ss -ltnp | grep -E ':(443|8080|4822|3389)' || true" >&2 || true
echo "---- Hatch processes ----" >&2
docker exec "${PREFIX}-hatch" ps aux >&2 || true
echo "---- Hatch logs ----" >&2
docker logs "${PREFIX}-hatch" >&2 || true
exit 1
