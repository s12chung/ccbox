FROM ghcr.io/jdx/mise:2026.6.9 AS mise

FROM debian:trixie-slim

# Each row is a section:
# - build toolchain
# - TLS roots for mise downloads
# - dev-workflow CLIs
# - Ruby runtime libs
# - Node runtime lib (V8 needs libatomic on arm64)
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        git curl less procps pkg-config unzip bind9-dnsutils \
        libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \
        libatomic1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

# System config (at /etc/mise) read by `mise install` below installs to shared /usr/local
# _temp_ MISE_DATA_DIR aims full install there  (`mise install --system` doesn't put shims)
# _temp_ so at runtime ccbox's `mise use` -> ~/.local.
COPY docker/mise-system.toml /etc/mise/config.toml
ENV PATH=/usr/local/share/mise/shims:$PATH
# Shared browser binaries for playwright (tests) and the desktop
# web-browser wrapper; outside the masked home caches, so every container
# from this image reuses one copy.
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
ENV NPM_CONFIG_PREFIX=/home/ccbox/.npm-global
ENV NPM_CONFIG_UPDATE_NOTIFIER=false
ENV GEM_HOME=/home/ccbox/.gem
ENV GOBIN=/home/ccbox/go/bin
# The clis volume's bin dir leads: ccboxtools installs the coding CLI there at start
ENV PATH=/opt/ccbox/clis/bin:/home/ccbox/.local/share/mise/shims:/home/ccbox/.local/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin:/home/ccbox/go/bin:$PATH

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

# Each row is a section — only the dep-graph's tips; everything else lands
# via Depends or playwright's install-deps (mise postinstall above):
# - libxss1, the one Electron/Chromium runtime lib for the deb-extracted
#   GUI app that playwright's dep list misses (libgbm1 etc. from the VNC
#   server below; libgtk-3-0t64 + libxtst6 land via the XFCE row, dejavu
#   fonts via fontconfig-config)
# - VNC server + passwd tool (Recommends of the server, which
#   --no-install-recommends skips), dbus-x11 for the session bus, xdg-utils
#   (slim lacks it — the OAuth chain needs it)
# - targeted XFCE: wm, session, desktop
# - the X session's base fonts (xfonts-base)
# TigerVNC serves an XFCE desktop over native VNC; docker/desktop.sh starts
# it in the foreground.
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
# it by absolute path; root-owned
COPY --chmod=755 docker/web-browser /usr/local/bin/web-browser
# Terminal helper: resizes the VNC box on demand, since clients without
# SetDesktopSize support (macOS Screen Sharing) can't move it themselves
COPY --chmod=755 docker/vncsize /usr/local/bin/vncsize

COPY --chmod=755 dist/ccboxtools /usr/local/bin/ccboxtools

# Build-time cleanup + groundwork for the volume mounts:
# - clean /root's build caches (mise, npm); /tmp restarts standard 1777
# - /opt/ is root owned, so change ownership
RUN rm -rf /root/.cache /root/.npm /tmp; mkdir -m 1777 /tmp; \
    mkdir -p /opt/ccbox/apps && chown -R ccbox:ccbox /opt/ccbox

USER ccbox

# Pre-create **generic** mountpoints so named volumes inherit uid 1000 (else root-owned, unwritable)
# - cacheVolumes - per-project caches mapped to pkg/dmap/mounts.go (/tmp is created above to keep 1777)
# - globalVolumes - global volumes mapped to pkg/dmap/mounts.go
# - /home/ccbox/.config - opencode's config dir's parent
# - git config ... - handles container user non-match repo owner problem - https://github.blog/open-source/git/git-security-vulnerability-announced/
RUN mkdir -p /home/ccbox/go /home/ccbox/.cache /home/ccbox/.gem \
             /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.local \
             /opt/ccbox/clis \
             /home/ccbox/.config && \
    git config --file /home/ccbox/.gitconfig --add safe.directory '*'

# data_bind_dirs is for pre-creating harness CLI mountpoints so named volumes inherit uid 1000 (else root-owned, unwritable)
ARG data_bind_dirs=""
# hadolint ignore=SC2086
RUN [ -z "$data_bind_dirs" ] || mkdir -p $data_bind_dirs

ENTRYPOINT ["/usr/local/bin/ccboxtools", "entrypoint"]
CMD ["bash"]