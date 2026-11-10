#!/bin/sh
# The image's desktop in two supervised halves: no argv execs the bare VNC
# server, `desktop session` execs the XFCE session on its display — the
# entrypoint respawns each on death.
set -eu

# Serve on display :0 to bind the stock RFB port (5900+N for :N) — the one
# macOS Screen Sharing connects to without spelling out the port.
export DISPLAY=:0
VNC_PW="${VNC_PW:-ccbox}"
VNC_RESOLUTION="${VNC_RESOLUTION:-1280x1024}"
VNC_COL_DEPTH="${VNC_COL_DEPTH:-24}"

mkdir -p "$HOME/.vnc"
# Pipe through filter mode to stay non-interactive — it also accepts short
# dev passwords.
printf '%s' "$VNC_PW" | tigervncpasswd -f > "$HOME/.vnc/passwd"
chmod 600 "$HOME/.vnc/passwd"

if [ "$#" -eq 0 ]; then
  # a SIGKILL'd predecessor leaves its locks on the persistent /tmp — this
  # server refuses the display over them
  rm -f "/tmp/.X${DISPLAY#:}-lock" "/tmp/.X11-unix/X${DISPLAY#:}"
  # no -auth: the session lands without an Xauthority file. -localhost=0: the
  # published port arrives on eth0 — exposure stays host-loopback-only
  # (pkg/docker/run.go), keeping the weak VncAuth password off the network
  exec /usr/bin/Xtigervnc "$DISPLAY" -localhost=0 \
    -geometry "$VNC_RESOLUTION" -depth "$VNC_COL_DEPTH" \
    -PasswordFile "$HOME/.vnc/passwd" -SecurityTypes VncAuth,TLSVnc
fi

# Append the session's output to the scratch log, off the boot's streams.
exec 1>>/tmp/desktop.log 2>&1

# The session landing: the bare server's display and the
# image's own session chain.
if [ "${1:-}" = session ]; then
  exec /etc/X11/Xtigervnc-session
fi

echo "desktop: unknown argv: ${1:-}" >&2
exit 2
