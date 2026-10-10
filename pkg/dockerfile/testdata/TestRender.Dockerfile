# The devbox image's Dockerfile template: `ccbox build` renders it into the build
# context (pkg/dockerfile) — the actions generate the runtime-owned statements
# from pkg/models/runtime.
FROM ghcr.io/jdx/mise:2026.6.9 AS mise

FROM debian:trixie-slim AS base

# Each row is a section:
# - build toolchain
# - TLS roots for mise downloads
# - dev-workflow CLIs
# - Node runtime lib (V8 needs libatomic on arm64)
# - Ruby runtime libs
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        git curl less procps pkg-config unzip bind9-dnsutils \
        libatomic1 \
        libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

# System config + optional lock (at /etc/mise) read by `mise install` below installs to shared /usr/local
# _temp_ MISE_DATA_DIR aims full install there  (`mise install --system` doesn't put shims)
# _temp_ so at runtime ccbox's `mise use` -> ~/.local.
COPY docker/mise/ /etc/mise/
ENV PATH=/usr/local/share/mise/shims:$PATH
# Shared browser binaries for playwright (tests) and the desktop variant's
# web-browser wrapper (the vnc stage below); outside the masked home caches,
# so every container from this image reuses one copy.
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright

# Ruby build headers: install, compile Ruby, then purge (runtime libs kept above)
# $buildDeps intentionally unquoted below so it word-splits into separate apt args
# HOME is pinned to /root so the install's npm junk (playwright) lands in
# the cleaned home, never the ccbox home the mounts share.
# hadolint ignore=SC2086
RUN set -eux; \
    export HOME=/root; \
    apt-get update; \
    buildDeps='libssl-dev libyaml-dev zlib1g-dev libffi-dev libreadline-dev libgmp-dev'; \
    apt-get install -y --no-install-recommends $buildDeps; \
    MISE_DATA_DIR=/usr/local/share/mise mise install; \
    apt-get purge -y --auto-remove $buildDeps; \
    rm -rf /var/lib/apt/lists/*

# Dedicated, unprivileged user. uid/gid 1000 matches the typical host user for bind-mount ownership.
RUN groupadd --gid 1000 ccbox && useradd --uid 1000 --gid ccbox --shell /bin/bash --create-home ccbox

ENV DEVCONTAINER=true

ENV TZ="America/New_York"

# Let ccbox install packages in every ecosystem with no root
ENV GEM_HOME=/home/ccbox/.gem
ENV GOBIN=/home/ccbox/go/bin
ENV NPM_CONFIG_PREFIX=/home/ccbox/.npm-global
ENV NPM_CONFIG_UPDATE_NOTIFIER=false
# The clis volume's bin dir leads: ccboxtools installs the coding CLI there at start
ENV PATH=/opt/ccbox/clis/bin:/home/ccbox/.local/share/mise/shims:/home/ccbox/.local/bin:/home/ccbox/go/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin:$PATH

# Two interactive-shell tweaks:
# - Debian's /etc/profile resets PATH on login shells (bash -l), clearing the append above.
# - readline gets zsh's AUTO_LIST+AUTO_MENU feel for `cl`<TAB>: first TAB lists matches
#   (claude, clear, ...), repeats cycle through them, Shift-TAB reverses.
RUN echo 'export PATH="'"$PATH"'"' > /etc/profile.d/ccbox-path.sh; \
    printf '%s\n' \
      'set show-all-if-ambiguous on' \
      'set menu-complete-display-prefix on' \
      'TAB: menu-complete' \
      '"\e[Z": menu-complete-backward' >> /etc/inputrc

COPY --chmod=755 dist/ccboxtools /usr/local/bin/ccboxtools

# Build-time cleanup + groundwork for the volume mounts:
# - clean /root's build caches (mise, npm); /tmp restarts standard 1777
# - /opt/ccbox hosts the volume mountpoints (the clis root below; the desktop
#   variant's apps root in the vnc stage)
RUN rm -rf /root/.cache /root/.npm /tmp; mkdir -m 1777 /tmp; \
    mkdir -p /opt/ccbox && chown ccbox:ccbox /opt/ccbox

USER ccbox

# Pre-create **generic** mountpoints so named volumes inherit uid 1000 (else root-owned, unwritable)
# - cacheVolumes - per-project caches mapped from pkg/models/runtime via pkg/dmap/mounts.go (/tmp is created above to keep 1777)
# - globalVolumes - global volumes mapped to pkg/dmap/mounts.go
# - /home/ccbox/.config - opencode's config dir's parent
# - git config ... - handles container user non-match repo owner problem - https://github.blog/open-source/git/git-security-vulnerability-announced/
RUN mkdir -p /home/ccbox/go /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.gem \
             /home/ccbox/.cache /home/ccbox/.local /home/ccbox/.config /opt/ccbox/clis && \
    git config --file /home/ccbox/.gitconfig --add safe.directory '*'

# data_bind_dirs is for pre-creating harness CLI mountpoints so named volumes inherit uid 1000 (else root-owned, unwritable)
ARG data_bind_dirs=""
# hadolint ignore=SC2086
RUN [ -z "$data_bind_dirs" ] || mkdir -p $data_bind_dirs

ENTRYPOINT ["/usr/local/bin/ccboxtools", "entrypoint"]
CMD ["bash"]

# The desktop variant: the headless base plus the VNC stack — TigerVNC serving
# XFCE (docker/desktop.sh starts it in the foreground), the session's browser
# wrapper, and the Electron/OAuth bits the desktop's GUI app needs.
FROM base AS vnc
# The stage inherits the base's ccbox USER; the apt row below needs root.
# hadolint ignore=DL3002
USER root

# Each row is a section — only the dep-graph's tips; everything else lands
# via Depends or the base's playwright browser install:
# - libxss1, the one Electron runtime lib for the deb-extracted GUI app that
#   the base's browser install misses (libgbm1 etc. from the VNC server in
#   this row; libgtk-3-0t64 + libxtst6 land via the XFCE row, dejavu fonts
#   via fontconfig-config)
# - VNC server + passwd tool (Recommends of the server, which
#   --no-install-recommends skips), dbus-x11 for the session bus, xdg-utils
#   (slim lacks it — the OAuth chain needs it)
# - targeted XFCE: wm, session, desktop
# - the X session's base fonts (xfonts-base)
RUN apt-get update && apt-get install -y --no-install-recommends \
        libxss1 \
        tigervnc-standalone-server tigervnc-tools dbus-x11 xdg-utils \
        xfwm4 xfce4-panel xfce4-session xfdesktop4 \
        xfonts-base \
    && rm -rf /var/lib/apt/lists/*

# The VNC session supervisor — its presence is the entrypoint's hook
# to start a GUI.
COPY --chmod=755 docker/desktop.sh /usr/local/bin/desktop
# The desktop's XDG pieces, baked: .desktop entries go to system dirs — .local
# is a per-project volume at runtime (pkg/dmap/mounts.go), which would hide
# home-baked copies — while .config is mount-free and ephemeral, so it bakes
# into the home directly.
COPY --chown=ccbox:ccbox docker/desktop/home/ /home/ccbox/
COPY docker/desktop/share/ /usr/local/share/
# The session's default browser — the baked mimeapps + xfce helpers reference
# it by absolute path; root-owned. Execs the base's shared playwright chromium.
COPY --chmod=755 docker/web-browser /usr/local/bin/web-browser

# The GUI app's volume mountpoint, ccbox-owned so the named volume seeds uid 1000
RUN mkdir -p /opt/ccbox/apps && chown ccbox:ccbox /opt/ccbox/apps

USER ccbox

# The default target
FROM base
