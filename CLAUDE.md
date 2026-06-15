## Overview

This project is a hardened Docker devbox wrapper for Claude Code - you are currently running inside of it, you already loaded its brief specifications in your managed-policy CLAUDE.md.

You run inside the container this Dockerfile builds, and we often swap containers as the Dockerfile changes, especially mid-debug — so the running image may not match the file on disk. Re-read the Dockerfile before answering "what's this line/file" (never reconstruct from git or memory), and when container/Dockerfile changes come up, ask whether the state is old or new.

### Key components
- **`Dockerfile`** — builds the devbox image from the inputs under `docker/`.
- **`docker/`**
  - `image/` — baked into the image:
    - `entrypoint.sh` — fail-closed start check: refuses to boot after user and network security checks
    - `mise-system.toml` — pinned system devbox toolchain (runtimes + CLIs), installed to `/etc/mise`.
  - `tinyproxy/` — the egress wall configs
  - `seed/` — managed policy by seed only: applied by the ccbox binary, so edits take effect on seeding, not in the running session
    - `claude-config/` — the Claude config
- **`tests/`**
- **`Makefile`** — primary entrypoints:
  - `make proxy` (needs docker) - spins up a `tinyproxy` egress container with the allowlist: `docker/tinyproxy/allow.txt` connected to an internal Docker network
  - `make run` (needs docker) - runs the `Dockerfile` connected to `make proxy`'s Docker network
  - `make lint` - linting
  - `make test.local` — runs all linting and tests that are possible without a Docker daemon