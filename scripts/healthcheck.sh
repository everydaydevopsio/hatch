#!/bin/sh
set -eu
pgrep -x nginx >/dev/null
pgrep -x guacd >/dev/null
pgrep -f 'org.apache.catalina.startup.Bootstrap' >/dev/null
pgrep -x xrdp >/dev/null
pgrep -x xrdp-sesman >/dev/null
grep -qi ':0D3D ' /proc/net/tcp /proc/net/tcp6 2>/dev/null
