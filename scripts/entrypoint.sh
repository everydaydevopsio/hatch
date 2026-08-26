#!/bin/sh
set -eu
RDP_USER="${RDP_USER:-oauth}"
case "$RDP_USER" in *[!a-zA-Z0-9_-]*|'') echo >&2 "ERROR: invalid RDP_USER."; exit 64;; esac
GENERATED_PASSWORD=0
if [ -z "${RDP_PASSWORD:-}" ]; then
  RDP_PASSWORD="$(openssl rand -hex 24)"
  GENERATED_PASSWORD=1
fi
case "$RDP_PASSWORD" in
  *:*|*'
'*) echo >&2 "ERROR: RDP_PASSWORD must not contain ':' or newlines."; exit 64;;
esac
if ! id "$RDP_USER" >/dev/null 2>&1; then useradd --create-home --shell /bin/bash "$RDP_USER"; fi
printf '%s:%s\n' "$RDP_USER" "$RDP_PASSWORD" | chpasswd
install -d -o "$RDP_USER" -g "$RDP_USER" -m 0700 "/home/$RDP_USER/.config"
install -d -o "$RDP_USER" -g "$RDP_USER" -m 0700 "/home/$RDP_USER/.cache"
install -d -m 0755 /etc/hatch
printf '%s\n' "${HATCH_START_URL:-about:blank}" > /etc/hatch/start-url
chmod 0644 /etc/hatch/start-url
install -d -m 0700 /var/log/hatch
{
  echo "Hatch RDP credentials"
  echo "Written: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "RDP user: $RDP_USER"
  echo "RDP password: $RDP_PASSWORD"
} > /var/log/hatch/rdp-credentials.log
chmod 0600 /var/log/hatch/rdp-credentials.log
if [ ! -s /etc/xrdp/key.pem ] || [ ! -s /etc/xrdp/cert.pem ]; then xrdp-keygen xrdp auto >/dev/null 2>&1 || true; fi
mkdir -p /run/xrdp
chmod 0755 /run/xrdp
rm -f /run/xrdp/xrdp.pid /run/xrdp/xrdp-sesman.pid
export RDP_USER RDP_PASSWORD HATCH_HTTPS_PORT HATCH_START_URL CHROMIUM_EXTRA_FLAGS GUACAMOLE_HOME GUACD_HOSTNAME GUACD_PORT WEBAPP_CONTEXT
/usr/local/bin/hatch-guacamole-config
echo "Hatch starting"
echo "RDP user: $RDP_USER"
if [ "$GENERATED_PASSWORD" -eq 1 ]; then
  echo "Generated RDP password: $RDP_PASSWORD"
  echo "Generated RDP credentials were also written to /var/log/hatch/rdp-credentials.log"
else
  echo "Using RDP password from RDP_PASSWORD"
  echo "RDP credentials were also written to /var/log/hatch/rdp-credentials.log"
fi
echo "HTTPS access: publish container port ${HATCH_HTTPS_PORT:-443}, for example -p 8443:${HATCH_HTTPS_PORT:-443}"
echo "OAuth callback note: use Docker host networking only when Chromium must reach services on host loopback"
exec "$@"
