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
case "${HATCH_MAC_SHORTCUTS:-1}" in
  1|true|TRUE|yes|YES|on|ON)
    if command -v xbindkeys >/dev/null 2>&1 && command -v xdotool >/dev/null 2>&1; then
      cat > "$XDG_CONFIG_HOME/hatch-mac-shortcuts.xbindkeysrc" <<'EOF'
"xdotool key --clearmodifiers ctrl+v"
  Mod4 + v
"xdotool key --clearmodifiers ctrl+c"
  Mod4 + c
"xdotool key --clearmodifiers ctrl+x"
  Mod4 + x
"xdotool key --clearmodifiers ctrl+a"
  Mod4 + a
"xdotool key --clearmodifiers ctrl+l"
  Mod4 + l
"xdotool key --clearmodifiers ctrl+t"
  Mod4 + t
"xdotool key --clearmodifiers ctrl+w"
  Mod4 + w
"xdotool key --clearmodifiers ctrl+r"
  Mod4 + r
"xdotool key --clearmodifiers ctrl+shift+v"
  Mod4 + Shift + v
EOF
      xbindkeys -f "$XDG_CONFIG_HOME/hatch-mac-shortcuts.xbindkeysrc" &
    fi
    ;;
esac
openbox-session &
OPENBOX_PID=$!
sleep 1
START_URL="$(cat /etc/hatch/start-url 2>/dev/null || printf '%s\n' about:blank)"
/usr/local/bin/hatch-chromium "$START_URL" &
xterm -geometry 100x28+20+20 -title "Hatch" -e /usr/local/bin/hatch-login-shell &
wait "$OPENBOX_PID"
