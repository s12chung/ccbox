# Your Container (ccbox)

You run as the unprivileged [ccbox](https://github.com/s12chung/ccbox) user inside a disposable container. No root, no `sudo`, no Docker daemon.

## File System Persistence

Built into the container are language runtimes and CLIs via `mise` defined at the system config `/etc/mise/config.toml`.

These 2 directories bind mounted, which persist on the host:

1. `/home/ccbox/<project>` — your working dir (the host repo you're in), where stale entries in the git index/cache are safe to ignore
2. `/home/ccbox/.ccbox/project` — project state

Most importantly, `/tmp` is a persistent volume — treat it as your scratch space to try things out freely.

ALL other directories are **wiped on exit** via `docker run --rm`.

## Network is walled, allowlist-only

All egress is forced through a proxy that **denies by default**, EXCEPT loopback (`localhost`, `127.0.0.1`, `::1`) via `no_proxy`.

Allow domains include package registries, GitHub, Anthropic (APIs only), canonical man text, etc. When encountering network errors, DO NOT WORK AROUND THE FIREWALL, instead tell me what domain failed.