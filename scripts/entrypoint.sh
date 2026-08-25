#!/bin/sh
set -eu
RDP_USER="${RDP_USER:-oauth}"
if [ -z "${RDP_PASSWORD:-}" ]; then echo >&2 "ERROR: RDP_PASSWORD is required."; exit 64; fi
case "$RDP_USER" in *[!a-zA-Z0-9_-]*|'') echo >&2 "ERROR: invalid RDP_USER."; exit 64;; esac
if ! id "$RDP_USER" >/dev/null 2>&1; then useradd --create-home --shell /bin/bash "$RDP_USER"; fi
printf '%s:%s\n' "$RDP_USER" "$RDP_PASSWORD" | chpasswd
install -d -o "$RDP_USER" -g "$RDP_USER" -m 0700 "/home/$RDP_USER/.config"
install -d -o "$RDP_USER" -g "$RDP_USER" -m 0700 "/home/$RDP_USER/.cache"
if [ ! -s /etc/xrdp/key.pem ] || [ ! -s /etc/xrdp/cert.pem ]; then xrdp-keygen xrdp auto >/dev/null 2>&1 || true; fi
mkdir -p /run/xrdp /run/dbus
chmod 0755 /run/xrdp
rm -f /run/xrdp/xrdp.pid /run/xrdp/xrdp-sesman.pid /run/dbus/pid
echo "Hatch starting"
echo "RDP user: $RDP_USER"
echo "Network requirement: Docker host networking for localhost OAuth callbacks"
exec "$@"
