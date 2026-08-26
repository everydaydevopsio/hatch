#!/bin/sh
set -eu
clear 2>/dev/null || true
cat <<'EOF'
          HATCH

           _________
        __/        /|
      _/__/_______/ |
     /   /       |  |
    /___/________|  /
    \   \        | /
     \___\_______|/
       \  open  /
        \______/

OAuth browser desktop is ready.
Chromium should open automatically.
EOF
echo
exec /bin/bash -l
