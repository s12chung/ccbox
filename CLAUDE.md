## Overview

This project is a hardened Docker devbox wrapper for Claude Code - you are currently running inside of it, you already loaded its brief specifications in your managed-policy CLAUDE.md.

You run inside the container this Dockerfile builds, and we often swap containers as the Dockerfile changes, especially mid-debug — so the running image may not match the file on disk. Re-read the Dockerfile before answering "what's this line/file" (never reconstruct from git or memory), and when container/Dockerfile changes come up, ask whether the state is old or new.

### Key components
The devbox lifecycle is driven by the **`ccbox`** Go CLI (cobra), which talks to the Docker Engine SDK in-process. Commands:
  - `ccbox` (no subcommand) — runs the devbox container interactively behind the wall, wiring the local terminal to the container's pty; launches `claude` by default (`--shell` for a plain shell)
  - `ccbox build` — builds the image from the embedded context on BuildKit (buildx library)
  - `ccbox proxy` — runs the `tinyproxy` egress wall in the foreground

- **`main.go`** — `//go:embed`s the build context (`Dockerfile`, `docker/image/*`) and proxy configs (`docker/tinyproxy/*`) into the binary, then hands off to `cmd`.
- **`cmd/`** — thin cobra commands: gather flags/env and call one `pkg/docker` operation each.
- **`pkg/`**
  - `docker/` — the build/run/proxy lifecycle over the Docker SDK
  - `prompt/` — interactive terminal I/O (ask on stderr, read stdin); the only place user prompts belong
  - `projectcfg/` — related to `.ccbox.yaml` from a workspace repo root, also contains any defaulting
  - `perm/` — named file/dir permission constants (`Dir`, `File`, `ExecFile`); use these, never bare octal
  - `log/` — log helpers and abstraction, never use `fmt.Print*`
- **`Dockerfile`** — builds the devbox image from the inputs under `docker/`.
- **`docker/`**
  - `image/` — baked into the image:
    - `entrypoint.sh` — fail-closed start check: refuses to boot after user and network security checks
    - `mise-system.toml` — pinned system devbox toolchain (runtimes + CLIs), installed to `/etc/mise`.
  - `tinyproxy/` — the egress wall configs
  - `seed/` — config, used only by `pkg/seed/`: it seeds these onto the host config dir and mounted to the container
    - `claude-config/` — the Claude config, `~/.ccbox/claude-config` → `~/ccbox/.ccbox/claude-config`
    - `project-slug/` — ccbox project data, `~/.ccbox/projects/-project-slug` (see below) → `~/ccbox/.ccbox/project`
- **`tests/`** — bats integration tests (need the built image; run by `make test.docker`).
- **`Makefile`** — primary entrypoints are:
  - `make build` — builds the `ccbox` binary to `/tmp/ccbox` in the **container**
  - `make lint` - all linting
  - `make test` — runs all linting and tests that are possible without a Docker daemon

### Project Slug
`docker.ProjectSlug` keys ccbox's per-project state: it slugifies the **host** cwd (e.g. `/Users/me/app` → `-Users-me-app`). Distinct from Claude Code's `claude-config/projects` slug, which CC derives from its **container** cwd.

### Go tests

Unit tests are colocated with their package (`*_test.go`). Assert with **`testify`**: `require` for preconditions that must hold before the test can continue (errors, setup), `assert` for the checks under test so a failure reports every mismatch.