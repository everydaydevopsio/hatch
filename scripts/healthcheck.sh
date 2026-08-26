#!/bin/sh
set -eu
pgrep -x nginx >/dev/null
pgrep -x guacd >/dev/null
pgrep -x xrdp >/dev/null
pgrep -x xrdp-sesman >/dev/null
HTTPS_PORT_HEX="$(printf '%04X' "${HATCH_HTTPS_PORT:-443}")"
grep -qi ":$HTTPS_PORT_HEX " /proc/net/tcp /proc/net/tcp6 2>/dev/null
grep -qi ':1F90 ' /proc/net/tcp /proc/net/tcp6 2>/dev/null
grep -qi ':12D6 ' /proc/net/tcp /proc/net/tcp6 2>/dev/null
grep -qi ':0D3D ' /proc/net/tcp /proc/net/tcp6 2>/dev/null
