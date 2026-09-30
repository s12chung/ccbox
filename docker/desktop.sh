#!/bin/sh
# Run the TigerVNC server in the foreground — hosting an XFCE session —
# to serve the GUI over VNC while the terminal session stays foreground
# in the entrypoint.
set -eu

# Serve on display :0 to bind the stock RFB port (5900+N for :N) — the one
# macOS Screen Sharing connects to without spelling out the port.
DISPLAY=:0
VNC_PW="${VNC_PW:-ccbox}"
VNC_RESOLUTION="${VNC_RESOLUTION:-1280x1024}"
VNC_COL_DEPTH="${VNC_COL_DEPTH:-24}"

mkdir -p "$HOME/.vnc"
# Pipe through filter mode to stay non-interactive — it also accepts short
# dev passwords.
printf '%s' "$VNC_PW" | tigervncpasswd -f > "$HOME/.vnc/passwd"
chmod 600 "$HOME/.vnc/passwd"

# Append to the scratch log to keep the entrypoint's streams for the session
# it serves.
exec 1>>/tmp/desktop.log 2>&1

# Restart the server on crash to keep the old stack's auto-recover; the trap
# kills it on exit so no stray server survives the script.
stopping=
trap 'stopping=1; tigervncserver -kill "$DISPLAY" 2>/dev/null || :' EXIT INT TERM
until [ "$stopping" ]; do
  # Clear the display's locks before every start to survive the persistent
  # /tmp shared across runs, because a server SIGKILL'd with its container
  # (no trap runs on teardown) leaves locks the next run's server refuses
  # the display over.
  rm -f "/tmp/.X${DISPLAY#:}-lock" "/tmp/.X11-unix/X${DISPLAY#:}"
  # Serve no custom xstartup to ride Debian's default chain —
  # /etc/X11/Xtigervnc-session execs Xsession, whose x-session-manager is
  # startxfce4 (the 75dbus snippet provides the session bus).
  # Listen beyond loopback to receive docker's published port, which arrives on
  # the container's eth0, not its loopback — exposure stays host-loopback-only
  # (pkg/docker/run.go publishes 5900 on 127.0.0.1), keeping the weak VncAuth
  # password off the network.
  tigervncserver -fg "$DISPLAY" -localhost no \
    -geometry "$VNC_RESOLUTION" -depth "$VNC_COL_DEPTH" \
    -rfbauth "$HOME/.vnc/passwd" &
  # Block in wait to give a trapped TERM an interruptible spot (a shell defers
  # traps behind foreground children), so the kill runs now instead of after
  # the server exits on its own.
  wait "$!" || :
  # Sleep a beat to keep a crash from spinning the restart loop.
  [ "$stopping" ] || sleep 2
done
