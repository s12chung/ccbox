## Overview

This project is a hardened Docker devbox wrapper for an LLM CLI — Claude Code or Codex, selected per project via `.ccbox.yaml`'s `cli` key. You are currently running inside of it, you already loaded its brief specifications in your managed-policy CLAUDE.md.

You run inside the container defined at `Dockerfile`, and we often swap containers as the Dockerfile changes, especially mid-debug — so the running image may not match the file on disk. When container/Dockerfile changes come up, ask whether the state is old or new.

### Key components
The devbox lifecycle is driven by the **`ccbox`** Go CLI (cobra) where golang files map to `cmd/`, which talks to the Docker Engine SDK in-process. Commands:
  - `ccbox` (no subcommand) — runs the devbox container interactively behind the wall, wiring the local terminal to the container's pty; launches the configured CLI by default (`--shell` for a plain shell, `--no-proxy` to skip the wall for direct egress). Maps to `cmd/run.go`.
  - `ccbox proxy` — runs the `tinyproxy` egress wall in the foreground
  - `ccbox config` - prints the effective .ccbox.yaml with tmpfsMasks, volumeMasks, and allowlist defaults applied

This curated directory will help you discover common patterns (`pkg/util` and `pkg/kit`) and navigate the project:
- **`main.go`** — `//go:embed`s the build context (`Dockerfile`, `docker/image/*`) and proxy configs (`docker/tinyproxy/*`) into the binary, then hands off to `cmd`.
- **`cmd/`** — thin cobra commands: gather flags/env and call one `pkg/docker` operation each.
- **`pkg/`**
  - `docker/` — the build/run/proxy lifecycle over the Docker SDK
  - `projectcfg/` — related to `ccbox` Config as described in the README
  - `harness/` — individual harness/cli related code; embeds its seed trees under `clis/`: it lays these onto the host config dir which is then mounted to the container. Only the configured CLI's config dir is seeded and mounted.
    - `shared/` — shared across CLIs: one `AGENTS.user.md`, renamed to the CLI's live memory file on seed (`CLAUDE.md` / `AGENTS.md`)
    - `(per-CLI directories)/` — each CLI's native config, `~/.ccbox/<cli>` → `~/ccbox/.<config>` (e.g. `.claude`)
  - `util/` - contains std lib utility packages
    - `httputil/` — http utilities for requests
    - `ioutil/` — io utils, including named file/dir permission constants (`Dir`, `File`, `ExecFile`); use these, never bare octal
    - `log/` — log helpers and abstraction, never use `fmt.Print*`
  - `kit/` - contains non-std lib abstractions and utilities
- **`Dockerfile`** — builds the devbox image from the inputs under `docker/`.
- **`docker/`**
  - `image/` — baked into the image:
    - `mise-system.toml` — pinned system devbox toolchain (runtimes + CLIs), installed to `/etc/mise`.
  - `tinyproxy/` — the egress wall configs
- **`tests/`** — bats integration tests (need the built image; run by `make test.docker`).
- **`Makefile`** — primary entrypoints are:
  - `make build` — builds the `ccbox` binary to `/tmp/ccbox` in the **container**
  - `make lint` - all linting
  - `make test` — runs all linting and tests that are possible without a Docker daemon

### Go tests

Unit tests are colocated with their package (`*_test.go`). Assert with **`testify`**: `require` for preconditions that must hold before the test can continue (errors, setup), `assert` for the checks under test so a failure reports every mismatch.

### Linting

`golangci-lint` is very strict. When encountering `bodyclose`, use this pattern `defer func() { log.WarnErr("intent", resp.Body.Close()) }()`, where `log` is `pkg/util/log`.