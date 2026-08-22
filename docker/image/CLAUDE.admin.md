# Your Container (ccbox)

You run as the unprivileged [ccbox](https://github.com/s12chung/ccbox) user inside a disposable container. No root, no `sudo`, no Docker daemon. The container is `--rm`: **everything is wiped on exit except these bind mounts**, which persist on the host:
- `/home/ccbox/<project>` — your working dir (the host repo you're in)
- `/home/ccbox/.ccbox/project` — per-project devbox state
- The agent harness CLI default config directory, which varies depending on harness CLI

## Persistent per-project volumes

There are other directories that are mounted as persistent per-project volumes:

- Generic paths: `~/.cache` and `~/.local`
- Runtime-specific caches and install directories, such as npm's `~/.npm` and `~/.npm-global` (Golang, Ruby, and Python are covered the same way)

Most importantly, `/tmp` is persistent too — treat it as your scratch space to try things out freely.

Any other directory is **ephemeral**, gone next run. You can still install per-user in any ecosystem with no root (e.g. `npm i -g`) — it survives only if it lands on a volume above.

## What's already installed
Recent build/base tooling are installed via `apt-get`. Language runtimes and CLIs via `mise` defined at the system config `/etc/mise/config.toml`. Useful CLIs to navigate around include `ripgrep`, `gh`, `jq`, and `yq`.

## Network: walled, allowlist-only
There is no general internet. All egress is forced through a proxy that **denies by default**; direct (non-proxy) traffic has no route at all. Loopback (`localhost`, `127.0.0.1`, `::1`) is pre-exempted from the proxy via `no_proxy`, so local dev servers and browsers reach it directly.

Allow domains include package registries, GitHub, Anthropic (APIs only), canonical man text, etc. This is configurable. When a request fails on the network, **tell me which domain it needed** so I can decide whether to add it. Don't silently work around it.

## Git

Stale entries in the git index/cache whose underlying file no longer exists are safe to ignore — IDEs are sharing this repo's `.git`, so leftover cache/lock artifacts from one side aren't real problems for the other.
