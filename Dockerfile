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
        git curl less procps pkg-config nano unzip bind9-dnsutils \
        libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \
        libatomic1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

# System config (at /etc/mise) read by `mise install` below installs to shared /usr/local
# _temp_ MISE_DATA_DIR aims full install there  (`mise install --system` doesn't put shims)
# _temp_ so at runtime ccbox's `mise use` -> ~/.local.
COPY docker/image/mise-system.toml /etc/mise/config.toml
ENV PATH=/usr/local/share/mise/shims:$PATH

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

# Dedicated, unprivileged user. uid/gid 1000 matches the typical host user for bind-mount ownership.
RUN groupadd --gid 1000 ccbox && useradd --uid 1000 --gid ccbox --shell /bin/bash --create-home ccbox
ENV DEVCONTAINER=true

# Managed-policy CLAUDE.md: org-wide memory, highest precedence, loaded every session for all users.
COPY docker/image/CLAUDE.admin.md /etc/claude-code/CLAUDE.md

# Make delta git's diff pager. --system writes /etc/gitconfig so it applies to all users.
RUN git config --system core.pager delta && git config --system interactive.diffFilter 'delta --color-only' && git config --system delta.navigate true
ENV TZ="America/New_York"
ENV EDITOR=nano
ENV VISUAL=nano

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
# so re-do the shim via. symlink.
ARG CLI=claude
ARG CLI_VERSION=latest
ARG PKGER
RUN set -eux; \
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
    rm -rf /root/.cache /tmp; mkdir -p /tmp/opencode; \
    chmod 1777 /tmp && chown ccbox:ccbox /tmp/opencode # standard 1777, force opencode CLI's scratch w-access (CLI makes it only root-w)

USER ccbox
# Pre-create cache mountpoints so per-project named volumes inherit uid 1000 (else root-owned, unwritable)
# Mapped to pkg/docker/mounts.go, `/tmp` is created above
RUN mkdir -p /home/ccbox/go /home/ccbox/.cache /home/ccbox/.gem \
             /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.local \
             /home/ccbox/.config # opencode uses .config

COPY --chmod=755 docker/image/entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash"]