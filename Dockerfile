# kasmweb core is Debian + the KasmVNC desktop stack (/dockerstartup): GUI apps are usable
# over VNC/web on 6901, started in the background by docker/image/desktop.sh.
# The base's last stage runs as uid 1000 — drop back to root for the system setup below.
FROM kasmweb/core-debian-bookworm:1.19.0
USER root

ARG TARGETARCH
ARG MISE_VERSION=2026.6.9

# Each row is a section:
# - build toolchain
# - TLS roots for mise downloads
# - dev-workflow CLIs
# - Ruby runtime libs
# - Node runtime lib (V8 needs libatomic on arm64)
# The base ships sudo, which the entrypoint fail-closed refuses — purge it.
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        git curl less procps pkg-config unzip bind9-dnsutils \
        libssl3 libyaml-0-2 zlib1g libffi8 libreadline8 libgmp10 libzstd1 \
        libatomic1 \
    # sudo's prerm refuses removal without this; the devbox wants exactly that
    && SUDO_FORCE_REMOVE=yes apt-get purge -y sudo \
    && rm -rf /var/lib/apt/lists/*

# mise, static musl build: the base's glibc (2.36) is older than the gnu builds require,
# and a static binary runs anywhere. Pinned to the version the mise-image stage carried.
RUN set -eux; \
    case "$TARGETARCH" in \
    amd64) arch=x64 ;; \
    arm64) arch=arm64 ;; \
    *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /usr/local/bin/mise "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-${arch}-musl"; \
    chmod 755 /usr/local/bin/mise

# System config (at /etc/mise) read by `mise install` below installs to shared /usr/local
# _temp_ MISE_DATA_DIR aims full install there  (`mise install --system` doesn't put shims)
# _temp_ so at runtime ccbox's `mise use` -> ~/.local.
COPY docker/image/mise-system.toml /etc/mise/config.toml
ENV PATH=/usr/local/share/mise/shims:$PATH
# Shared browser binaries for playwright (tests) and the desktop web-browser wrapper;
# outside the masked home caches, so every container from this image reuses one copy.
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/ms-playwright

# Ruby build headers: install, compile Ruby, then purge (runtime libs kept above)
# $buildDeps intentionally unquoted below so it word-splits into separate apt args
# hadolint ignore=SC2086
RUN set -eux; \
    apt-get update; \
    buildDeps='libssl-dev libyaml-dev zlib1g-dev libffi-dev libreadline-dev libgmp-dev'; \
    apt-get install -y --no-install-recommends $buildDeps; \
    MISE_DATA_DIR=/usr/local/share/mise mise install; \
    apt-get purge -y --auto-remove $buildDeps; \
    rm -rf /var/lib/apt/lists/*

# Desktop app: install the vendor .deb (Electron; apt resolves its deps) for the target
# arch. Electron's sandbox needs setuid/user-namespaces, both gone in the devbox, so a
# wrapper pins --no-sandbox and the .desktop launcher routes through it too. Autostart in
# the VNC session rides the desktop stack's custom hook (docker/image/zcode-autostart.sh).
ARG ZCODE_VERSION=3.11.2
RUN set -eux; \
    case "$TARGETARCH" in \
    amd64) appArch=x64 ;; \
    arm64) appArch=arm64 ;; \
    *) echo "unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/app.deb "https://cdn-zcode.z.ai/zcode/electron/releases/${ZCODE_VERSION}/linux-${appArch}/ZCode-${ZCODE_VERSION}-linux-${appArch}.deb"; \
    apt-get update; \
    apt-get install -y --no-install-recommends /tmp/app.deb; \
    rm -f /tmp/app.deb; \
    printf '#!/bin/sh\nexec /opt/ZCode/zcode --no-sandbox "$@"\n' > /usr/local/bin/zcode; \
    chmod 755 /usr/local/bin/zcode; \
    sed -i 's|^Exec=.*|Exec=zcode --no-sandbox %U|' /usr/share/applications/zcode.desktop; \
    sed -i 's|^Exec=.*|Exec=/usr/local/bin/web-browser %U|' /usr/share/applications/xfce4-web-browser.desktop; \
    rm -rf /var/lib/apt/lists/*
COPY --chmod=755 docker/image/zcode-autostart.sh /dockerstartup/custom_startup.sh
# desktop(1) is the entrypoint's hook: present = the image serves a GUI over VNC
COPY --chmod=755 docker/image/desktop.sh /usr/local/bin/desktop
# The desktop's web client serves plain HTTP: the port publishes on the host loopback
# only, so TLS buys nothing while its self-signed cert trips browser warnings. The yaml
# keeps the literal ${HOME} — KasmVNC interpolates it at runtime. The startup script's
# auth user becomes ccboxvnc; the password rides VNC_PW (default in desktop.sh).
# hadolint ignore=SC2016
RUN sed -i -e 's/ -sslOnly//g' -e 's/kasm_user/ccboxvnc/g' /dockerstartup/vnc_startup.sh \
    && printf '%s\n' \
    'network:' \
    '  ssl:' \
    '    pem_certificate: ${HOME}/.vnc/self.pem' \
    '    pem_key: ${HOME}/.vnc/self.pem' \
    '    require_ssl: false' \
    '  udp:' \
    '    public_ip: 127.0.0.1' \
    > /etc/kasmvnc/kasmvnc.yaml \
    && printf '%s\n' '[Desktop Entry]' 'Name=Web Browser' 'Exec=/usr/local/bin/web-browser %U' \
    'Terminal=false' 'Type=Application' 'Icon=web-browser' 'Categories=Network;WebBrowser;' \
    'MimeType=x-scheme-handler/http;x-scheme-handler/https;text/html;' \
    > /usr/share/applications/web-browser.desktop
COPY --chmod=755 docker/image/web-browser /usr/local/bin/web-browser

# Dedicated, unprivileged user. uid/gid 1000 matches the typical host user for bind-mount
# ownership; the desktop stack's stock user already holds the uid, so rename it in place
# and move its home to the ccbox layout. Re-running the profile step populates the home
# from the stack's template (re-owned to uid 1000) and re-points the VNC web root's
# Downloads symlink; the sweep rewrites home configs still holding the old path. Pipeline
# status is xargs's, so grep's no-match exit is fine to drop. The bashrc line's $STARTUPDIR
# is quoted literally — it's expanded by the shell that sources the line at runtime.
# hadolint ignore=DL4006,SC2016
RUN groupmod -n ccbox kasm-user \
    && usermod -l ccbox -s /bin/bash -d /home/ccbox -m kasm-user \
    && rm -f /home/ccbox/.bashrc \
    && HOME=/home/ccbox bash /dockerstartup/kasm_default_profile.sh true \
    && echo 'source $STARTUPDIR/generate_container_user' >> /home/ccbox/.bashrc \
    && chown -R ccbox:ccbox /home/ccbox \
    && grep -rl /home/kasm-user /home/ccbox | xargs -r sed -i s#/home/kasm-user#/home/ccbox#g

# Runtime merges the image env with the devbox's minimal env (only explicit overrides
# win), and the base carries HOME=/home/kasm-user — re-point it at the moved home.
ENV HOME=/home/ccbox
# The web login is ccboxvnc/ccboxvnc: the base's VNC_PW=vncpassword would otherwise win
# the merge over desktop.sh's default (password only; the user is patched at build).
ENV VNC_PW=ccboxvnc
# Same for the base's WORKDIR: left stale it makes docker create /home/kasm-user (root-
# owned parent, uid-1000 dir) in every container built or spawned from this image —
# including ccbox's throwaway chown helpers.
WORKDIR /home/ccbox
ENV DEVCONTAINER=true

# Managed-policy CLAUDE.md: org-wide memory, highest precedence, loaded every session for all users.
COPY docker/image/CLAUDE.admin.md /etc/claude-code/CLAUDE.md

ENV TZ="America/New_York"

# Let ccbox install packages in every ecosystem with no root
ENV NPM_CONFIG_PREFIX=/home/ccbox/.npm-global
ENV NPM_CONFIG_UPDATE_NOTIFIER=false
ENV GEM_HOME=/home/ccbox/.gem
ENV GOBIN=/home/ccbox/go/bin
ENV PATH=/home/ccbox/.local/share/mise/shims:/home/ccbox/.local/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin:/home/ccbox/go/bin:$PATH

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

# Coding CLI last for version bumping and clear mise cache.
#
# PKGER encodes the install source, rendered by pkg/pkger for the configured CLI:
# - npm:<package>
# - versionurl:<latest_version_url>|<linux-x64-url>|<linux-arm64-url>, whose urls embed a
#   literal $version swapped in below once CLI_VERSION resolves
#
# npm_args: --ignore-scripts=false re-runs the postinstall mise's npm backend skips;
# --allow-scripts adds it to npm 11's separate allowlist, else npm warns each install.
#
# http backend (versionurl): bin renames the raw binary. mise can't reverse-resolve its own shim
# without MISE_DATA_DIR in the env (which we leave unset so ccbox's `mise use` targets ~/.local),
# so re-do the shim via. symlink. HOME is pinned to /root: the image-level HOME is the
# ccbox home, and root's npm/mise junk must not land there (it's cleaned up below, along
# with any /home/kasm-user the install recreated).
ARG CLI=claude
ARG CLI_VERSION=latest
ARG PKGER
RUN set -eux; \
    export HOME=/root; \
    cfg=/etc/mise/config.toml; \
    scheme="${PKGER%%:*}"; \
    pkgData="${PKGER#*:}"; \
    case "$scheme" in \
    npm) \
        tool="npm:$pkgData"; \
        mise config set --file "$cfg" "tools.$tool.version" "${CLI_VERSION}"; \
        mise config set --file "$cfg" "tools.$tool.npm_args" -- "--ignore-scripts=false --allow-scripts=$pkgData"; \
        ;; \
    versionurl) \
        latest_url="${pkgData%%|*}"; pair="${pkgData#*|}"; \
        x64_tpl="${pair%%|*}"; arm64_tpl="${pair#*|}"; \
        ver="$CLI_VERSION"; \
        if [ "$ver" = latest ]; then ver=$(curl -fsSL "$latest_url"); fi; \
        tool="http:$CLI"; \
        mise config set --file "$cfg" "tools.$tool.version" "$ver"; \
        mise config set --file "$cfg" "tools.$tool.bin" "$CLI"; \
        mise config set --file "$cfg" "tools.$tool.platforms.linux-x64.url" "${x64_tpl%%\$version*}$ver${x64_tpl#*\$version}"; \
        mise config set --file "$cfg" "tools.$tool.platforms.linux-arm64.url" "${arm64_tpl%%\$version*}$ver${arm64_tpl#*\$version}"; \
        ;; \
    *) echo "unknown pkger scheme: $scheme" >&2; exit 1;; \
    esac; \
    MISE_DATA_DIR=/usr/local/share/mise mise install "$tool"; \
    if [ "$scheme" = versionurl ]; then ln -sf "/usr/local/share/mise/installs/http-$CLI/latest/$CLI" "/usr/local/share/mise/shims/$CLI"; fi; \
    rm -rf /root/.cache /root/.npm /home/kasm-user /tmp; mkdir -p /tmp/opencode; \
    chmod 1777 /tmp && chown ccbox:ccbox /tmp/opencode # standard 1777, force opencode CLI's scratch w-access (CLI makes it only root-w)

USER ccbox
# Pre-create cache mountpoints so per-project named volumes inherit uid 1000 (else root-owned, unwritable)
# Mapped to pkg/docker/mounts.go, `/tmp` is created above
RUN mkdir -p /home/ccbox/go /home/ccbox/.cache /home/ccbox/.gem \
             /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.local \
             /home/ccbox/.config # opencode uses .config
# Default web browser for GUI apps' auth flows: xdg/exo otherwise fail ("Failed to execute
# default Web Browser"), and the stock epiphany .desktop would sandbox-crash — so both
# point at the wrapper above.
RUN printf '%s\n' '[Default Applications]' \
      'x-scheme-handler/http=web-browser.desktop' \
      'x-scheme-handler/https=web-browser.desktop' \
      'text/html=web-browser.desktop' > /home/ccbox/.config/mimeapps.list \
    && mkdir -p /home/ccbox/.config/xfce4 \
    && printf '%s\n' '[Default Applications]' 'WebBrowser=web-browser' \
      > /home/ccbox/.config/xfce4/helpers.rc

COPY --chmod=755 docker/image/entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash"]