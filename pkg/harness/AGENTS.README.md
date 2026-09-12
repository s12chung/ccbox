# Shared AGENTS docs

`AGENTS.user.md` is shared amongst all CLIs dynamically. For example, with claude, it is mounted to `~/.claude/CLAUDE.md`. If any changes are made to your `CLAUDE.md` during a `ccbox run`, that changed copy will be placed in `claude/CLAUDE.md`.

`AGENTS.user.ccbox-admin.md` is converted to `AGENTS.user.md` with ccbox's admin level instructions prepended. If both files are deleted, `AGENTS.user.ccbox-admin.md` will be restored at `ccbox run`. Also, `claude/CLAUDE.ccbox-admin.md` will convert to `claude/CLAUDE.md` too, with the admin instructions prepended.