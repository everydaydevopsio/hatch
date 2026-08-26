#!/usr/bin/env bash
set -euo pipefail

IMAGE="${HATCH_E2E_IMAGE:-hatch:e2e}"
PREFIX="${HATCH_E2E_NAME:-hatch-e2e}"
NETWORK="${PREFIX}-net"
GUAC_PORT="${HATCH_E2E_GUAC_PORT:-18080}"
URL="${HATCH_E2E_URL:-https://www.google.com}"
RDP_USER="${HATCH_E2E_RDP_USER:-oauth}"
GUAC_VERSION="${HATCH_E2E_GUAC_VERSION:-1.6.0}"
POSTGRES_IMAGE="${HATCH_E2E_POSTGRES_IMAGE:-postgres:16-alpine}"
DB_NAME="guacamole_db"
DB_USER="guacamole_user"
DB_PASSWORD="guacamole_pass"
TMP_DIR="$(mktemp -d)"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: $1 is required." >&2
    exit 2
  fi
}

cleanup() {
  docker rm -f "${PREFIX}-guacamole" "${PREFIX}-guacd" "${PREFIX}-postgres" "${PREFIX}-hatch" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_command docker
require_command nc
require_command node
require_command npm

cleanup
mkdir -p "$TMP_DIR"
docker network create "$NETWORK" >/dev/null

docker build -t "$IMAGE" .

docker run -d \
  --name "${PREFIX}-hatch" \
  --network "$NETWORK" \
  --shm-size=1g \
  --security-opt no-new-privileges:true \
  -e RDP_USER="$RDP_USER" \
  -e HATCH_START_URL="$URL" \
  "$IMAGE" >/dev/null

for _ in $(seq 1 30); do
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

RDP_PASSWORD="$(docker logs "${PREFIX}-hatch" 2>&1 | sed -n 's/^Generated RDP password: //p' | tail -n 1)"
if [ -z "$RDP_PASSWORD" ]; then
  docker logs "${PREFIX}-hatch" >&2 || true
  echo "ERROR: Could not extract generated RDP password from Hatch logs." >&2
  exit 1
fi

docker run -d \
  --name "${PREFIX}-postgres" \
  --network "$NETWORK" \
  -e POSTGRES_DB="$DB_NAME" \
  -e POSTGRES_USER="$DB_USER" \
  -e POSTGRES_PASSWORD="$DB_PASSWORD" \
  "$POSTGRES_IMAGE" >/dev/null

for _ in $(seq 1 45); do
  if docker exec "${PREFIX}-postgres" pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker exec "${PREFIX}-postgres" pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
  docker logs "${PREFIX}-postgres" >&2 || true
  echo "ERROR: Postgres did not become ready." >&2
  exit 1
fi

docker run --rm "guacamole/guacamole:${GUAC_VERSION}" /opt/guacamole/bin/initdb.sh --postgresql > "$TMP_DIR/initdb.sql"
docker exec -i "${PREFIX}-postgres" psql -U "$DB_USER" -d "$DB_NAME" < "$TMP_DIR/initdb.sql" >/dev/null

cat > "$TMP_DIR/connection.sql" <<SQL
INSERT INTO guacamole_connection (connection_name, protocol)
VALUES ('Hatch', 'rdp');

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'hostname', '${PREFIX}-hatch'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'port', '3389'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'username', '${RDP_USER}'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'password', '${RDP_PASSWORD}'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'ignore-cert', 'true'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, 'security', 'any'
FROM guacamole_connection
WHERE connection_name = 'Hatch';

INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
SELECT e.entity_id, c.connection_id, 'READ'
FROM guacamole_entity e
CROSS JOIN guacamole_connection c
WHERE e.name = 'guacadmin'
  AND c.connection_name = 'Hatch';
SQL
docker exec -i "${PREFIX}-postgres" psql -U "$DB_USER" -d "$DB_NAME" < "$TMP_DIR/connection.sql" >/dev/null

docker run -d \
  --name "${PREFIX}-guacd" \
  --network "$NETWORK" \
  "guacamole/guacd:${GUAC_VERSION}" >/dev/null

docker run -d \
  --name "${PREFIX}-guacamole" \
  --network "$NETWORK" \
  -p "127.0.0.1:${GUAC_PORT}:8080" \
  -e GUACD_HOSTNAME="${PREFIX}-guacd" \
  -e POSTGRESQL_HOSTNAME="${PREFIX}-postgres" \
  -e POSTGRESQL_DATABASE="$DB_NAME" \
  -e POSTGRESQL_USER="$DB_USER" \
  -e POSTGRESQL_PASSWORD="$DB_PASSWORD" \
  "guacamole/guacamole:${GUAC_VERSION}" >/dev/null

for _ in $(seq 1 60); do
  if nc -z 127.0.0.1 "$GUAC_PORT" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! nc -z 127.0.0.1 "$GUAC_PORT" >/dev/null 2>&1; then
  docker logs "${PREFIX}-guacamole" >&2 || true
  echo "ERROR: Guacamole did not open port $GUAC_PORT." >&2
  exit 1
fi

cat > "$TMP_DIR/guac-smoke.mjs" <<'JS'
import { chromium } from 'playwright';

const baseUrl = process.env.GUAC_URL;

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });

await page.goto(`${baseUrl}/guacamole/`, { waitUntil: 'domcontentloaded' });
await page.getByLabel(/username/i).fill('guacadmin');
await page.getByLabel(/password/i).fill('guacadmin');
await page.getByRole('button', { name: /login/i }).click();

await page.getByText('Hatch', { exact: true }).waitFor({ timeout: 30000 });
await page.getByText('Hatch', { exact: true }).click();
await page.locator('canvas').first().waitFor({ timeout: 60000 });

await page.waitForTimeout(5000);

await browser.close();
JS

GUAC_URL="http://127.0.0.1:${GUAC_PORT}" \
  sh -c 'cd "$1" && npm init -y >/dev/null && npm install playwright@1.57.0 >/dev/null && npx playwright install chromium >/dev/null && node guac-smoke.mjs' sh "$TMP_DIR"

for _ in $(seq 1 45); do
  if docker exec "${PREFIX}-hatch" sh -lc "pgrep -af 'chromium.*${URL}' >/dev/null"; then
    echo "Guacamole E2E succeeded: browser login reached Hatch RDP and Chromium opened $URL"
    exit 0
  fi
  sleep 1
done

echo "ERROR: Chromium process for $URL was not observed in Hatch." >&2
echo "---- Hatch processes ----" >&2
docker exec "${PREFIX}-hatch" ps aux >&2 || true
echo "---- Hatch logs ----" >&2
docker logs "${PREFIX}-hatch" >&2 || true
echo "---- guacd logs ----" >&2
docker logs "${PREFIX}-guacd" >&2 || true
echo "---- Guacamole logs ----" >&2
docker logs "${PREFIX}-guacamole" >&2 || true
exit 1
