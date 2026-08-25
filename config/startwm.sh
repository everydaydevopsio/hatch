#!/bin/sh
set -eu
USER_NAME="${USER:-${LOGNAME:-oauth}}"
HOME_DIR="$(getent passwd "$USER_NAME" | cut -d: -f6)"
export HOME="${HOME_DIR:-/home/$USER_NAME}"
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_CACHE_HOME="$HOME/.cache"
export XDG_RUNTIME_DIR="/tmp/runtime-$USER_NAME"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"
openbox-session &
OPENBOX_PID=$!
sleep 1
/usr/local/bin/hatch-chromium &
xterm -geometry 100x28+20+20 -title "Hatch" &
wait "$OPENBOX_PID"
