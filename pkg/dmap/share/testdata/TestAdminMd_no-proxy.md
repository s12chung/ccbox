# Your Container (ccbox)

You run as the unprivileged [ccbox](https://github.com/s12chung/ccbox) user inside a disposable container. No root, no `sudo`, no Docker daemon.

## File System Persistence

Built into the container are language runtimes and CLIs via `mise` defined at the system config `/etc/mise/config.toml`.

These 2 directories bind mounted, which persist on the host:

1. `/home/ccbox/<project>` — your working dir (the host repo you're in)
2. `/home/ccbox/.ccbox/persist` — persistent per-project state, known as the "persist directory/folder"

Most importantly, `/tmp` is a persistent volume — treat it as your scratch space to try things out freely.

ALL other directories are **wiped on exit** via `docker run --rm`.
