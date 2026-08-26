#!/bin/sh
set -eu

check_listen_port() {
  port_hex="$(printf '%04X' "$1")"
  awk -v port="$port_hex" '
    NR > 1 {
      split($2, local, ":")
      if (toupper(local[2]) == port && $4 == "0A") {
        found = 1
      }
    }
    END {
      exit found ? 0 : 1
    }
  ' /proc/net/tcp /proc/net/tcp6
}

pgrep -x nginx >/dev/null
pgrep -x guacd >/dev/null
pgrep -x xrdp >/dev/null
pgrep -x xrdp-sesman >/dev/null
check_listen_port "${HATCH_HTTPS_PORT:-443}"
check_listen_port 8080
check_listen_port 4822
check_listen_port 3389
