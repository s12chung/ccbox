# ccbox

> A protected Docker devbox for running a harness CLI — Claude Code, Codex, etc.

Run `ccbox` from a repo root to run the harness CLI in a Docker container. `ccbox --shell` will open a shell instead.

- Limits harness access — unprivileged user, no root, no Docker daemon, a disposable container
- Configurable [protective host-mounts](#protective-mounts) to the container
- Full control over harness configuration, including adding your own

## Config

Your first run of a harness CLI will create the CLI's config folder in `~/.ccbox`.

The `ccbox` config scope hierarchy is:

- User - `~/.ccbox/config/ccbox.yaml` (no dot) — seeded with ccbox's defaults on first run
- Project - `project_dir/.ccbox.yaml`
- Project Local - `project_dir/.ccbox.local.yaml` (for git ignore)
- `ccbox` flags

`ccbox config` prints the effective config with `ccbox-defaults` expanded. `ccbox config init` creates a commented empty config. When merging configs, arrays and maps are merged.

## Protective Mounts

Protect your protect these configs:

- `tmpfs_masks` — hide directories behind a throwaway temp filesystem. For editor junk (`.vscode`) or throwaway build output.
- `volume_masks` — cover directories with a persistent per-project volume: the container keeps its own copy across runs. For `node_modules`-like folders.
- `read_only_globs` — block writes to glob paths (files or directories) with read-only re-mounts: the real files, never modified. For files that must never change (configs, secrets).

`ccbox` built-in LLM harness configurations prevent reads, as external container configurations can't. You can [add your own harnesses](pkg/harness/user-clis/README.md) that do so too.

## Docs

- **[`docker/image/CLAUDE.admin.md`](docker/image/CLAUDE.admin.md)** — start here: the container the CLI runs inside (user, network wall, what's installed).
- **[`AGENTS.md`](AGENTS.md)** — the project layout: the `ccbox` CLI, the build inputs, and how to build/test.
