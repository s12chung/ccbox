---
name: add-user-cli
description: Add or override a coding CLI in ccbox's user CLI dir (~/.ccbox/config/clis/<name>/)
---

# Add a CLI

1. From the CLI's own docs, collect: how it installs (an npm package, or versioned download
   URLs), its launch command, its continue/resume flags (watch for picker-vs-named-resume
   quirks), its native config dir, and the domains it talks to.
2. Create `~/.ccbox/config/clis/<name>/CLI.yaml` by copying the closest
   [built-in](https://github.com/s12chung/ccbox/tree/main/pkg/harness/clis). Fields are strict:
   unknown ones are rejected at startup. Comments in the built-ins flag the gotchas (e.g. which
   CLIs need `config_dir_env_key`, resume-flag quirks).
3. Optional: add `<name>/config/` with seed files, laid onto the CLI's config dir on first run
   (see claude's for hardening examples).
4. Run `ccbox --cli <name>`. An invalid CLI.yaml is skipped with a startup warning — fix and
   rerun; nothing else needs a rebuild.

The name is the directory name (lowercase). A user CLI overrides a same-name built-in.
