# Shared AGENTS docs

`AGENTS.user.md` is shared amongst all CLIs dynamically. For example, with claude, `~/.claude/AGENTS.md` is a symlink to `~/.ccbox/tmp/claude/AGENTS.md`, which is mounted into the container at `ccbox run`. If any changes are made to it during the run, that changed copy will be placed in `claude/AGENTS.md`.

`AGENTS.user.ccbox-admin.md` is converted to `AGENTS.user.md` with ccbox's admin level instructions prepended. If both files are deleted, `AGENTS.user.ccbox-admin.md` will be restored at `ccbox run`. Also, `claude/AGENTS.ccbox-admin.md` will convert to `claude/AGENTS.md` too, with the admin instructions prepended.