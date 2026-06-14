FROM ghcr.io/jdx/mise:2026.6.9 AS mise

FROM node:26-trixie-slim

# Each row is a section:
# - build toolchain
# - TLS roots for mise downloads
# - dev-workflow CLIs
# - Ruby runtime libs
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        git curl less procps pkg-config nano unzip bind9-dnsutils \
        libssl3t64 libyaml-0-2 zlib1g libffi8 libreadline8t64 libgmp10 libzstd1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

# System config (at /etc/mise) read by `mise install` below installs to shared /usr/local
# _temp_ MISE_DATA_DIR aims full install there  (`mise install --system` doesn't put shims)
# _temp_ so at runtime ccbox's `mise use` -> ~/.local.
COPY mise-system.toml /etc/mise/config.toml
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
    rm -rf /var/lib/apt/lists/* /root/.cache

ARG CLAUDE_CODE_VERSION=latest
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}

# Make delta git's diff pager. --system writes /etc/gitconfig so it applies to all users.
RUN git config --system core.pager delta && git config --system interactive.diffFilter 'delta --color-only' && git config --system delta.navigate true

ENV TZ="America/New_York"
ENV CLAUDE_CONFIG_DIR=/home/ccbox/claude-config
ENV CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
ENV DISABLE_AUTOUPDATER=1
ENV DEVCONTAINER=true
ENV EDITOR=nano
ENV VISUAL=nano

# Replace base image's 'node' account with a dedicated, unprivileged 'ccbox' user.
# Claiming uid/gid 1000 preserves bind-mount ownership behavior.
RUN userdel -r node \
    && groupadd --gid 1000 ccbox \
    && useradd --uid 1000 --gid ccbox --shell /bin/bash --create-home ccbox

# Let ccbox install packages in every ecosystem with no root
ENV NPM_CONFIG_PREFIX=/home/ccbox/.npm-global
ENV GEM_HOME=/home/ccbox/.gem
ENV GOBIN=/home/ccbox/go/bin
ENV PATH=/home/ccbox/.local/share/mise/shims:/home/ccbox/.local/bin:/home/ccbox/.npm-global/bin:/home/ccbox/.gem/bin:/home/ccbox/go/bin:$PATH
USER ccbox
WORKDIR /home/ccbox/workspace

COPY --chmod=755 entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash"]