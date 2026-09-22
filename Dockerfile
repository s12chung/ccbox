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

COPY --chmod=755 dist/ccboxtools /usr/local/bin/ccboxtools

# Build-time cleanup + groundwork for the volume mounts:
# - clean /root's build caches from mise; /tmp restarts standard 1777
# - /opt/ is root owned, so change ownership
RUN rm -rf /root/.cache /tmp; mkdir -m 1777 /tmp; \
    mkdir -p /opt/ccbox && chown ccbox:ccbox /opt/ccbox

USER ccbox

# Pre-create mountpoints so named volumes inherit uid 1000 (else root-owned, unwritable)
# - cacheVolumes - per-project caches mapped to pkg/docker/mounts.go (/tmp is created above to keep 1777)
# - globalVolumes - global volumes mapped to pkg/docker/mounts.go
# - /home/ccbox/.config /home/ccbox/.local/share/ - opencode uses it
# - /tmp/opencode - opencode's scratch (opencode CLI creates it root-only otherwise)
# - git config ... - handles container user non-match repo owner problem - https://github.blog/open-source/git/git-security-vulnerability-announced/
RUN mkdir -p /home/ccbox/go /home/ccbox/.cache /home/ccbox/.gem \
             /home/ccbox/.npm /home/ccbox/.npm-global /home/ccbox/.local \
             /opt/ccbox/clis \
             /home/ccbox/.config /home/ccbox/.local/share/opencode /tmp/opencode && \
    git config --file /home/ccbox/.gitconfig --add safe.directory '*'

ENTRYPOINT ["/usr/local/bin/ccboxtools", "entrypoint"]
CMD ["bash"]