# ccbox

A hardened Docker devbox for running an LLM CLI — Claude Code or Codex, selected per project.

## Usage

Run `ccbox` from a repo root: it builds the image if needed, then drops you into the devbox behind the egress wall and launches the configured CLI. Use `ccbox --shell` to open a shell instead. Use `ccbox --no-proxy` to run without the egress wall — direct network access, no allowlist.

## Philosophy: lock down everything, trust the mount

The premise is to **limit the CLI's access as much as possible** — unprivileged user, no root, no Docker daemon, a disposable `--rm` container, and an allowlist-only egress wall that denies network by default.

The one deliberate exception is **what you mount**. Whatever you bind-mount into the devbox container is fully the CLI's to read, write, and act on. **The mount is the trust boundary** — we don't gate reads inside it. If you mount a secret, the CLI has the secret, so mount only what you're willing to expose.

As a safety net for accidents, the CLI is also seeded to refuse reading common secret files (`.env`, `*.pem`, `*.key`, `secrets/`) and to ask before shell commands touch them. These don't add protection beyond the mount — anything inside it can still be reached.

## Config

The config scope hierarchy is:

- User - `~/.ccbox/config/ccbox.yaml` (no dot) — seeded with ccbox's defaults on first run
- Project - `project_dir/.ccbox.yaml`
- Project Local - `project_dir/.ccbox.local.yaml` (for git ignore)
- `ccbox` flags

`ccbox config` prints the effective config with `ccbox-defaults` expanded. `ccbox config init` creates a commented empty config. When merging configs, arrays and maps are merged.

## Docs

- **[`docker/image/CLAUDE.admin.md`](docker/image/CLAUDE.admin.md)** — start here: the container the CLI runs inside (user, network wall, what's installed).
- **[`AGENTS.md`](AGENTS.md)** — the project layout: the `ccbox` CLI, the build inputs, and how to build/test.
