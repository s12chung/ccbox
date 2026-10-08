## Overview

This project is a hardened Docker devbox wrapper for an LLM CLI. You are currently running inside of it.

You run inside the container defined at `Dockerfile`, and we often swap containers as the Dockerfile changes, especially mid-debug — so the running image may not match the file on disk. When container/Dockerfile changes come up, ask whether the state is old or new.

### Key components
The devbox lifecycle is driven by the **`ccbox`** Go CLI (cobra) where golang files map to `cmd/`, which talks to the Docker Engine SDK in-process. Commands:
- `ccbox` (no subcommand) — runs the devbox container interactively behind the egress wall, wiring the local terminal to the container's pty; launches the configured CLI by default (`ccbox run [command...]` for a plain shell or a one-off command, `--no-proxy` to skip the proxy for direct egress). Maps to `cmd/run.go`.
- `ccbox proxy` — runs the `tinyproxy` egress wall in the foreground
- `ccbox config` — prints the effective .ccbox.yaml

This curated directory will help you discover common patterns (`pkg/util` and `pkg/kit`) and navigate the project:
- **`main.go`** — `//go:embed`s the build context (`Dockerfile`, `docker/*`) plus the prebuilt `dist/ccboxtools` binary into the binary, then hands off to `cmd`.
- **`tools/`** — everything that builds and ships the container's tool binary (`dist/ccboxtools`)
  - `ccboxtools/` — the container's entrypoint — it verifies the container security and installs and maintains the harness CLI (nested Go module, replace'd in go.mod)
    - `pkg/util/log` — log helpers and abstraction, never use `fmt.Print*`
  - `toolsbuild/` — `go run` command that builds `dist/ccboxtools`
  - `build/` — the canonical build of the `ccboxtools` module, shared by `toolsbuild` and `ccbox doctor tools`
- **`cmd/`** — thin cobra commands: gather flags/env, map them to options via `pkg/dmap`, and call one `pkg/docker` operation each.
- **`pkg/`**
  - `dmap/` — maps the projectcfg.Config, CLI, and run flags to the docker pkg options for a run. The run's shares are wired at `dmap/share` — see its `AGENTS.README.md` for the AGENTS docs wiring TLDR.
  - `docker/` — the build/run/proxy lifecycle over the Docker SDK--manages DOCKER ONLY, knows nothing but docker lifecycle
  - `mise/` — related to mise configs
  - `projectcfg/` — related to `ccbox` Config as described in the README
  - `cli/` — individual cli related code: the registry + CLI.yaml parsing. Built-in CLI templates are `go:embed` at `clitmpl/clis/` and user configurable at `userdir.Dir()/clis` with the same format as the built-ins.
  - `userdir/` — resolves ccbox's per-user directory (`~/.ccbox`) for configs and persistent storage
  - `util/` — std lib utility packages, notable: `must`, `fsync`, `slug`, `mergeempty`, `uslice`
    - `httputil/` — http utilities for requests
    - `ioutil/` — io utils, including named file/dir permission constants (`Dir`, `File`, `ExecFile`); use these, never bare octal
  - `kit/` — non-std lib abstractions and utilities, most used: `dock`, `pick`, `firmrule`
    - `tinyproxy/` — the egress wall configs
- **`Dockerfile`** — builds the devbox image from the inputs under `docker/` plus mise config defined in the `mise` pkg.
- **`docker/`** — baked into the vnc variant image only
- **`tests/`** — bats integration tests (need the built image; run by `make test.docker`).
- **`Makefile`** — primary entrypoints are:
  - `make build` — builds `dist/ccboxtools` via `tools/toolsbuild`, verifies it with `doctor tools`, then builds the `ccbox` binary to `/tmp/ccbox` in the **container**
  - `make lint` - all linting
  - `make test` — runs all linting and tests that are possible without a Docker daemon

### Conventions

Terminology:

- A **path** is a single path string - container paths end in `Mount`; host paths are unmarked. A **mount** is a bind, volume, or tmpfs mount, even when typed as an interface, refer to binds as "binds" and volumes as "volumes".
- `tmpfs_masks`, `volume_masks`, and `read_only_globs` are internally termed as **guardMounts**. The term is code-only — never show it to users. When code handles all three, keep them in the stated order: `tmpfs_masks`, `volume_masks`, and `read_only_globs`.

For tests:

- Name tests `Test<Subject>_<Case>`, splitting multi-level names (e.g. `TestSeedUserConfig_SkipsExisting`, `TestFS_Rename_CrossKind`, not `TestSeedUserConfigSkipsExisting`); a lone subject needs no case (`TestMinus`).
- Place tests of the same subject next to each other — the name's subject tells you where a test goes.
- Ensure ALL errors are asserted or required, never write `_ = returnsErr()`
- Table tests are standard for test cases. For complex cases, use `t.Run()`, never comments.

### Linting

`golangci-lint` is very strict. When encountering `bodyclose`, use this pattern `defer func() { log.WarnErr("intent", resp.Body.Close()) }()`, where `log` is `tools/ccboxtools/pkg/util/log`.