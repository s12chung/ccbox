#!/usr/bin/env bash
# Autostarts the desktop app in the VNC session. The desktop stack reruns this script if
# it exits, so keep it alive after backgrounding the app.
set -euo pipefail

sleep 3 # let the window manager settle, else the app window lands unmanaged
zcode >/tmp/desktop-app.log 2>&1 &
sleep infinity
