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
# Config mount path: ccbox passes this as a build arg (single source in pkg/docker);
# the default keeps a bare `docker build` working.
ARG CLAUDE_CONFIG_DIR=/home/ccbox/.ccbox/claude-config
ENV CLAUDE_CONFIG_DIR=${CLAUDE_CONFIG_DIR}
ENV CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
ENV DISABLE_AUTOUPDATER=1

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

# Claude Code last for version bumping and clear mise cache
# npm_args: --ignore-scripts=false re-runs the postinstall mise's npm backend skips;
# --allow-scripts adds it to npm 11's separate allowlist, else npm warns each install.
ARG CLAUDE_CODE_VERSION=latest
RUN set -eux; \
    cfg=/etc/mise/config.toml; pkg='@anthropic-ai/claude-code'; tool="npm:$pkg"; \
    mise config set --file "$cfg" "tools.$tool.version" "${CLAUDE_CODE_VERSION}"; \
    mise config set --file "$cfg" "tools.$tool.npm_args" -- "--ignore-scripts=false --allow-scripts=$pkg"; \
    MISE_DATA_DIR=/usr/local/share/mise mise install "$tool"; \
    rm -rf /root/.cache

USER ccbox
# Pre-create cache mountpoints so per-project named volumes inherit uid 1000 (else root-owned, unwritable)
# Mapped to pkg/docker/run.go
RUN mkdir -p /home/ccbox/go /home/ccbox/.cache /home/ccbox/.gem \
             /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.local

# Default workdir for bare `docker build`/`docker run`; ccbox overrides it per-project
# at run time (sets the container's working dir to /home/ccbox/<host-dir-basename>).
WORKDIR /home/ccbox/workspace

COPY --chmod=755 docker/image/entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash"]