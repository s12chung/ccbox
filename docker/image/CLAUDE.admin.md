# Your Container (ccbox)

You run as the unprivileged `ccbox` user inside a disposable container. No root, no `sudo`, no Docker daemon. The container is `--rm`: **everything outside your project bind-mount (your `~/` working dir) is wiped on exit.**

## Network: walled, allowlist-only
There is no general internet. All egress is forced through a proxy that **denies by default**; direct (non-proxy) traffic has no route at all. Allow domains include package registries, GitHub, Anthropic (APIs only), canonical man text, and possibly more.

When a request fails on the network, **tell me which domain it needed** so I can decide whether to add it. Don't silently work around it.

## Installing things
You *can* install per-user in any ecosystem with no root — `npm i -g`, `pip install --user`, `gem install`, `go install`, `mise use`. But:
- **Prefer not to.** Reach for what's already here first.
- Anything you install is **ephemeral** — gone next run. Say so when you install, and flag it as a candidate for the Dockerfile if it should persist.

## Docs / man pages
No `man`. Fetch pages on demand from these allowlisted mirrors: `manpages.debian.org`, `man7.org`, `man.cx`, `linux.die.net`, `manpages.ubuntu.com` (e.g. `curl https://manpages.debian.org/bookworm/manpages/tar.1.en.txt` for plain text).

## What's already installed
Recent versions: node/npm from the base image, build/base tooling via `apt-get`, runtimes and CLIs
via `mise` defined at the system config `/etc/mise/config.toml`.

- **Runtimes**: `go`, `python`, `ruby`, `node`/`npm`.
- **CLIs**: `rg` (ripgrep), `fd`, `jq`, `yq`, `gh`, `bats`, `hadolint`, `shellcheck`.
- **Base**: `git`, `curl`, `unzip`, `dig` (debug the wall), and a build toolchain (`gcc`, `make`, `pkg-config`).
