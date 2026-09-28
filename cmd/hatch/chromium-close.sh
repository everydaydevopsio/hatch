#!/bin/sh
set -eu

browser_running() {
  [ -r "/proc/$1/stat" ] || return 1
  state=$(awk '{ print $3 }' "/proc/$1/stat")
  [ "$state" != Z ] && [ "$state" != X ]
}

wait_for_browser() {
  pid=$1
  attempts=$2
  while [ "$attempts" -gt 0 ]; do
    browser_running "$pid" || return 0
    sleep 0.25
    attempts=$((attempts - 1))
  done
  ! browser_running "$pid"
}

for desktop_pid in $(pgrep -u "$(id -u)" -x hatch-startwm || true); do
  browser_pids=$(pgrep -P "$desktop_pid" -x chromium || true)
  [ -n "$browser_pids" ] || continue

  display=$(tr '\000' '\n' < "/proc/$desktop_pid/environ" | sed -n 's/^DISPLAY=//p' | head -n 1)
  if [ -n "$display" ]; then
    DISPLAY="$display" XAUTHORITY="$HOME/.Xauthority" \
      xdotool search --onlyvisible --class chromium windowclose >/dev/null 2>&1 || true
  fi

  for browser_pid in $browser_pids; do
    wait_for_browser "$browser_pid" 40 && continue
    kill -TERM "$browser_pid" 2>/dev/null || true
    wait_for_browser "$browser_pid" 20 && continue
    kill -KILL "$browser_pid" 2>/dev/null || true
    wait_for_browser "$browser_pid" 8 || exit 124
  done
done

# A successful close means no live Chromium process remains in this container.
for browser_pid in $(pgrep -u "$(id -u)" -x chromium || true); do
  browser_running "$browser_pid" && exit 124
done
exit 0
