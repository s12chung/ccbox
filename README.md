# ccbox

A hardened Docker devbox for running Claude Code.

## Usage

Run `ccbox` from a repo root: it builds the image if needed, then drops you into the devbox behind the egress wall and launches `claude`. Use `ccbox --shell` to open a shell instead of Claude.

## Philosophy: lock down everything, trust the mount

The premise is to **limit Claude's access as much as possible** — unprivileged user, no root, no Docker daemon, a disposable `--rm` container, and an allowlist-only egress wall that denies network by default.

The one deliberate exception is **what you mount**. Whatever you bind-mount into the devbox container is fully Claude's to read, write, and act on. **The mount is the trust boundary** — we don't gate reads inside it. If you mount a secret, Claude has the secret, so mount only what you're willing to expose.

As a safety net for accidents, Claude is also seeded to refuse reading common secret files (`.env`, `*.pem`, `*.key`, `secrets/`) and to ask before shell commands touch them. These don't add protection beyond the mount — anything inside it can still be reached.

## Per-project config (`.ccbox.yaml`)

`ccbox` has an internal default. `ccbox config` prints the effective config with defaults applied. `ccbox config init` creates a documented default.

## Docs

- **[`docker/image/CLAUDE.admin.md`](docker/image/CLAUDE.admin.md)** — start here: the container Claude runs inside (user, network wall, what's installed).
- **[`CLAUDE.md`](CLAUDE.md)** — the project layout: the `ccbox` CLI, the build inputs, and how to build/test.
