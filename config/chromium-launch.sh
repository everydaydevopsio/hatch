#!/bin/sh
set -eu
FLAGS="--disable-dev-shm-usage --no-first-run --no-default-browser-check --disable-session-crashed-bubble"
if grep -q '^NoNewPrivs:[[:space:]]*1$' /proc/self/status 2>/dev/null; then
  FLAGS="$FLAGS --no-sandbox --test-type"
fi
if [ "$#" -eq 0 ]; then set -- about:blank; fi
# shellcheck disable=SC2086
exec /usr/bin/chromium $FLAGS ${CHROMIUM_EXTRA_FLAGS:-} "$@"
