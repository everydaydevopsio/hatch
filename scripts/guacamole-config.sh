#!/bin/sh
set -eu

json_escape() {
  printf '%s' "$1" \
    | sed \
      -e 's/\\/\\\\/g' \
      -e 's/"/\\"/g'
}

base64url() {
  openssl base64 -A | tr '+/' '-_' | tr -d '='
}

jwt_signing_input() {
  header="$(printf '{"alg":"HS512","typ":"JWT"}' | base64url)"
  payload="$(printf '%s' "$1" | base64url)"
  printf '%s.%s' "$header" "$payload"
}

create_jwt() {
  payload="$1"
  signing_input="$(jwt_signing_input "$payload")"
  signature="$(
    printf '%s' "$signing_input" \
      | openssl dgst -sha512 -hmac "$HATCH_GUAC_JWT_SECRET" -binary \
      | base64url
  )"
  printf '%s.%s' "$signing_input" "$signature"
}

install -d -m 0755 /etc/guacamole /etc/guacamole/extensions /etc/guacamole/lib /etc/hatch/tls /etc/nginx/conf.d

TLS_CERT="${HATCH_TLS_CERT:-/etc/hatch/tls/hatch.crt}"
TLS_KEY="${HATCH_TLS_KEY:-/etc/hatch/tls/hatch.key}"
TLS_CN="${HATCH_TLS_CN:-hatch.local}"
HATCH_GUAC_LAUNCH_TTL_SECONDS="${HATCH_GUAC_LAUNCH_TTL_SECONDS:-43200}"

if [ ! -s "$TLS_CERT" ] || [ ! -s "$TLS_KEY" ]; then
  openssl req \
    -x509 \
    -newkey rsa:2048 \
    -sha256 \
    -days "${HATCH_TLS_DAYS:-365}" \
    -nodes \
    -subj "/CN=$TLS_CN" \
    -keyout "$TLS_KEY" \
    -out "$TLS_CERT" >/dev/null 2>&1
  chmod 0600 "$TLS_KEY"
  chmod 0644 "$TLS_CERT"
fi

cat > /etc/nginx/conf.d/hatch.conf <<EOF
server {
    listen ${HATCH_HTTPS_PORT:-443} ssl;
    server_name _;

    ssl_certificate $TLS_CERT;
    ssl_certificate_key $TLS_KEY;
    ssl_protocols TLSv1.2 TLSv1.3;
    absolute_redirect off;
    error_page 497 =301 https://\$http_host\$request_uri;

    access_log /dev/stdout;
    error_log /dev/stderr warn;

    location = / {
        return 302 /hatch/;
    }

    location = /guacamole/ {
        if (\$arg_token = "") {
            return 302 /hatch/;
        }

        proxy_pass http://127.0.0.1:8080/guacamole/;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
    }

    location = /hatch/ {
        root /etc;
        try_files /hatch/guacamole-launcher.html =404;
        default_type text/html;
    }

    location /guacamole/ {
        proxy_pass http://127.0.0.1:8080/guacamole/;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
    }
}
EOF

cat > /etc/nginx/conf.d/connection_upgrade.conf <<'EOF'
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
EOF

if [ -z "${HATCH_GUAC_JWT_SECRET:-}" ]; then
  HATCH_GUAC_JWT_SECRET="$(openssl rand -hex 32)"
fi
if [ "${#HATCH_GUAC_JWT_SECRET}" -lt 32 ]; then
  echo >&2 "ERROR: HATCH_GUAC_JWT_SECRET must be at least 32 characters."
  exit 64
fi
case "$HATCH_GUAC_LAUNCH_TTL_SECONDS" in
  ''|*[!0-9]*) echo >&2 "ERROR: HATCH_GUAC_LAUNCH_TTL_SECONDS must be a positive integer."; exit 64;;
esac
if [ "$HATCH_GUAC_LAUNCH_TTL_SECONDS" -le 0 ]; then
  echo >&2 "ERROR: HATCH_GUAC_LAUNCH_TTL_SECONDS must be a positive integer."
  exit 64
fi

JWT_EXP="$(($(date -u +%s) + HATCH_GUAC_LAUNCH_TTL_SECONDS))"
CONNECTION_ID="Hatch"
CLIENT_ID="$(printf '%s\0c\0jwt' "$CONNECTION_ID" | base64url)"
HATCH_GUAC_JWT="$(
  create_jwt "$(printf '{"GUAC_ID":"%s","exp":%s,"guac.protocol":"rdp","guac.hostname":"127.0.0.1","guac.port":"3389","guac.username":"%s","guac.password":"%s","guac.security":"any","guac.ignore-cert":"true","guac.resize-method":"display-update"}' \
    "$(json_escape "$CONNECTION_ID")" \
    "$JWT_EXP" \
    "$(json_escape "$RDP_USER")" \
    "$(json_escape "$RDP_PASSWORD")")"
)"

cat > /etc/guacamole/guacamole.properties <<EOF
guacd-hostname: ${GUACD_HOSTNAME:-127.0.0.1}
guacd-port: ${GUACD_PORT:-4822}
secret-key: ${HATCH_GUAC_JWT_SECRET}
EOF

cat > /etc/hatch/guacamole-launcher.html <<EOF
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Hatch</title>
  <style>
    body { align-items: center; background: #0f172a; color: #f8fafc; display: flex; font-family: system-ui, sans-serif; height: 100vh; justify-content: center; margin: 0; }
    main { max-width: 28rem; padding: 1.5rem; text-align: center; }
    button { background: #38bdf8; border: 0; border-radius: .375rem; color: #082f49; cursor: pointer; font: inherit; font-weight: 700; padding: .75rem 1rem; }
    p { color: #cbd5e1; }
  </style>
</head>
<body>
  <main>
    <h1>Opening Hatch</h1>
    <p id="status">Starting a browser desktop session.</p>
    <button id="retry" hidden>Retry</button>
  </main>
  <script>
    const clientId = "$(json_escape "$CLIENT_ID")";

    async function launch() {
      const jwtToken = (new URLSearchParams(window.location.search).get("token") || "").replace(/\s+/g, "");
      if (!jwtToken) {
        document.getElementById("status").textContent = "Open Hatch with the generated access URL from the container logs.";
        return;
      }

      document.getElementById("retry").hidden = true;
      document.getElementById("status").textContent = "Starting a browser desktop session.";
      const body = new URLSearchParams({ token: jwtToken });
      const response = await fetch("/guacamole/api/tokens", {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body
      });

      if (!response.ok) {
        throw new Error("Guacamole token request failed with " + response.status);
      }

      const session = await response.json();
      const token = encodeURIComponent(session.authToken);
      window.location.replace("/guacamole/?token=" + token + "&GUAC_DATA_SOURCE=jwt#/client/" + clientId);
    }

    launch().catch((error) => {
      document.getElementById("status").textContent = error.message;
      document.getElementById("retry").hidden = false;
    });
    document.getElementById("retry").addEventListener("click", launch);
  </script>
</body>
</html>
EOF

chmod 0644 /etc/guacamole/guacamole.properties
chmod 0644 /etc/hatch/guacamole-launcher.html

echo "Hatch HTTPS endpoint: container port ${HATCH_HTTPS_PORT:-443}"
echo "Hatch access URL: https://<host>:${HATCH_HTTPS_PORT:-443}/hatch/?token=$HATCH_GUAC_JWT"
echo "Hatch access path: /hatch/?token=$HATCH_GUAC_JWT"
echo "Guacamole authentication: jwt"
