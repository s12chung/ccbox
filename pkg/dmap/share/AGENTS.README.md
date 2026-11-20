# Shared AGENTS docs

`AGENTS.tmpl.md` is a Go template (delimiters `<!--{{` and `}}-->`), shared amongst all CLIs dynamically. For example, with claude, `~/.claude/AGENTS.md` is a symlink to `~/.ccbox/tmp/agents/claude/AGENTS.md`, which is mounted into the container at boot. If any changes are made to it during the run, that changed copy will be placed in `claude/AGENTS.md`.

The template funcs: `adminSection` (ccbox's admin doc), `proxySection` (its proxied-only network section), `IsProxied`, and `sharedAgentsMd` — the shared doc's own rendered content, available only in a CLI's template. The seeded default is the admin section plus the proxied section when proxied; edit `AGENTS.tmpl.md` to taste. If it is deleted, it is restored at boot.

A CLI's own `~/.ccbox/<cli>/AGENTS.tmpl.md` wins over the shared one — use `<!--{{ sharedAgentsMd }}-->` in it to compose the shared content with CLI-specific rules.
