# Your Container (ccbox)

You run as the unprivileged [ccbox](https://github.com/s12chung/ccbox) user inside a disposable container. No root, no `sudo`, no Docker daemon. The container is `--rm`: **everything is wiped on exit except these bind mounts**, which persist on the host:
- `/home/ccbox/<project>` — your working dir (the host repo you're in)
- `/home/ccbox/.ccbox/claude-config` — Claude config
- `/home/ccbox/.ccbox/project` — per-project devbox state (lessons, bug notes)

Other dirs also live on persistent per-project volumes that survive across sessions: for Node, the workspace's `node_modules` plus caches like `~/.npm` and `~/.npm-global`. The same holds for the other runtimes.

## Installing: what persists
You can install per-user in any ecosystem with no root (e.g. `npm i -g`). It survives only if it lands in a persistent volume above, as global installs do; anything else is **ephemeral**, gone next run.

## What's already installed
Recent versions: node/npm from the base image, build/base tooling via `apt-get`, runtimes and CLIs
via `mise` defined at the system config `/etc/mise/config.toml`.

- **Runtimes**: `go`, `python`, `ruby`, `node`/`npm`.
- **CLIs**: `rg` (ripgrep), `fd`, `jq`, `yq`, `gh`, `bats`, `hadolint`, `shellcheck`.
- **Base**: `git`, `curl`, `unzip`, `dig` (debug the wall), and a build toolchain (`gcc`, `make`, `pkg-config`).
- **Browser**: `playwright` with Chromium system libs (no browser binaries, see Network section)

## Network: walled, allowlist-only
There is no general internet. All egress is forced through a proxy that **denies by default**; direct (non-proxy) traffic has no route at all. Loopback (`localhost`, `127.0.0.1`, `::1`) is pre-exempted from the proxy via `no_proxy`, so local dev servers and browsers reach it directly.

Allow domains include package registries, GitHub, Anthropic (APIs only), canonical man text, etc. This is configurable at `allowlist.domains` in the repo's `.ccbox.yaml`. When a request fails on the network, **tell me which domain it needed** so I can decide whether to add it. Don't silently work around it.

## Docs / man pages
No `man`. Fetch pages on demand from these allowlisted mirrors: `manpages.debian.org`, `man7.org`, `man.cx`, `linux.die.net`, `manpages.ubuntu.com` (e.g. `curl https://manpages.debian.org/bookworm/manpages/tar.1.en.txt` for plain text).

### `.ccbox.yaml` is off-limits
`.ccbox.yaml` contains configurations related to devbox security. Don't read or change it unless I ask: `Write`/`Edit` are denied and a Bash tripwire prompts on it.
