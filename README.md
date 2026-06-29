# ccbox

A hardened Docker devbox for running Claude Code.

## Usage

Run `ccbox` from a repo root: it builds the image if needed, then drops you into the devbox behind the egress wall and launches `claude`. Use `ccbox --shell` to open a shell instead of Claude.

## Philosophy: lock down everything, trust the mount

The premise is to **limit Claude's access as much as possible** — unprivileged user, no root, no Docker daemon, a disposable `--rm` container, and an allowlist-only egress wall that denies network by default.

The one deliberate exception is **what you mount**. Whatever you bind-mount into the devbox container is fully Claude's to read, write, and act on. **The mount is the trust boundary** — we don't gate reads inside it. If you mount a secret, Claude has the secret, so mount only what you're willing to expose.

As a safety net for accidents, Claude is also seeded to refuse reading common secret files (`.env`, `*.pem`, `*.key`, `secrets/`) and to ask before shell commands touch them. These don't add protection beyond the mount — anything inside it can still be reached.

## Per-project config (`.ccbox.yaml`)

Drop a `.ccbox.yaml` at a repo root to configure `ccbox`; a missing file changes nothing.

```yaml
# tmpfs: workspace-relative dirs masked with a writable, executable tmpfs, so in-container
# writes never reach the host bind-mount — handy for OS-specific build outputs (e.g. a
# dist/ Go binary that differs between a macOS host and the Linux container).
tmpfs:
  - .idea
  - dist

# env: extra environment variables set in the container (cannot override the proxy/token vars).
env:
  GOFLAGS: -mod=mod

# allowlist: domains the egress wall lets through. `domains` are added on top of the built-in
# defaults (package registries, GitHub, Anthropic APIs, man mirrors)
allowlist:
  defaults: true
  domains:
    - example.com
    - test.mywebsite.com
```

## Docs

- **[`docker/image/CLAUDE.admin.md`](docker/image/CLAUDE.admin.md)** — start here: the container Claude runs inside (user, network wall, what's installed).
- **[`CLAUDE.md`](CLAUDE.md)** — the project layout: the `ccbox` CLI, the build inputs, and how to build/test.
