#!/bin/sh
set -eu

xml_escape() {
  printf '%s' "$1" \
    | sed \
      -e 's/&/\&amp;/g' \
      -e 's/</\&lt;/g' \
      -e 's/>/\&gt;/g' \
      -e 's/"/\&quot;/g' \
      -e "s/'/\&apos;/g"
}

install -d -m 0755 /etc/guacamole /etc/hatch/tls /etc/nginx/conf.d

TLS_CERT="${HATCH_TLS_CERT:-/etc/hatch/tls/hatch.crt}"
TLS_KEY="${HATCH_TLS_KEY:-/etc/hatch/tls/hatch.key}"
TLS_CN="${HATCH_TLS_CN:-hatch.local}"

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

    access_log /dev/stdout;
    error_log /dev/stderr warn;

    location = / {
        return 302 https://\$http_host/guacamole/;
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

GUAC_USER="${GUAC_USER:-$RDP_USER}"
GUAC_PASSWORD="${GUAC_PASSWORD:-$RDP_PASSWORD}"

cat > /etc/guacamole/guacamole.properties <<EOF
guacd-hostname: ${GUACD_HOSTNAME:-127.0.0.1}
guacd-port: ${GUACD_PORT:-4822}
user-mapping: /etc/guacamole/user-mapping.xml
EOF

cat > /etc/guacamole/user-mapping.xml <<EOF
<user-mapping>
  <authorize username="$(xml_escape "$GUAC_USER")" password="$(xml_escape "$GUAC_PASSWORD")">
    <connection name="Hatch">
      <protocol>rdp</protocol>
      <param name="hostname">127.0.0.1</param>
      <param name="port">3389</param>
      <param name="username">$(xml_escape "$RDP_USER")</param>
      <param name="password">$(xml_escape "$RDP_PASSWORD")</param>
      <param name="security">any</param>
      <param name="ignore-cert">true</param>
      <param name="resize-method">display-update</param>
    </connection>
  </authorize>
</user-mapping>
EOF

chmod 0600 /etc/guacamole/user-mapping.xml
chmod 0644 /etc/guacamole/guacamole.properties

echo "Hatch HTTPS endpoint: container port ${HATCH_HTTPS_PORT:-443}"
echo "Guacamole URL path: /guacamole/"
echo "Guacamole user: $GUAC_USER"
echo "Guacamole password: $GUAC_PASSWORD"
