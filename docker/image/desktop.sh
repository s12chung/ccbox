#!/usr/bin/env bash
# Serves the GUI over VNC: starts the desktop stack (X server + window manager + VNC/web
# client on 6901) in the background, so the terminal session stays in the foreground. The
# stack's env lives here, not in the image ENV — the devbox replaces the image env with a
# minimal one (pkg/docker/env.go), and the shell profile only re-adds the PATH.
set -euo pipefail

# Desktop stack env (kasmweb core-image defaults; override per run, e.g. VNC_RESOLUTION)
export \
  STARTUPDIR=/dockerstartup \
  KASM_VNC_PATH=/usr/share/kasmvnc \
  DISPLAY=:1 \
  VNC_PORT=5901 NO_VNC_PORT=6901 \
  VNC_PW="${VNC_PW:-ccboxvnc}" VNC_VIEW_ONLY_PW="${VNC_VIEW_ONLY_PW:-vncviewonlypassword}" \
  VNC_COL_DEPTH=24 VNC_RESOLUTION="${VNC_RESOLUTION:-1280x1024}" MAX_FRAME_RATE=24 \
  VNCOPTIONS="-PreferBandwidth -DynamicQualityMin=4 -DynamicQualityMax=7 -DLP_ClipDelay=0" \
  KASMVNC_AUTO_RECOVER=true \
  START_XFCE4=1 \
  LANG="${LANG:-C.UTF-8}"
export LD_LIBRARY_PATH="/opt/libjpeg-turbo/lib64:/usr/local/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

# kasm_default_profile.sh execs vnc_startup.sh, which starts the stack, monitors it and
# keeps the session alive until signaled; logs land in the container's scratch space.
exec "$STARTUPDIR/kasm_default_profile.sh" "$STARTUPDIR/vnc_startup.sh" --wait >>/tmp/desktop.log 2>&1
