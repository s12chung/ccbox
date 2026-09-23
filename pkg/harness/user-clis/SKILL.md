---
name: add-user-cli
description: Add or override a coding CLI in ccbox's user CLI dir (~/.ccbox/config/clis/<name>/)
---

# Add a CLI

1. Create `~/.ccbox/config/clis/<name>/CLI.yaml` by copying [claude's](https://github.com/s12chung/ccbox/tree/main/pkg/harness/clis/claude) — the most complete built-in, with comment documentation. Fill the fields in from the CLI's own docs. Fields are strict: unknown ones are rejected at startup.
2. Optional: add `<name>/config/` with seed files, laid onto the CLI's config dir on first run — claude's includes a secrets tripwire (a PreToolUse hook) and a statusline.
3. `ccbox doctor clis` reports load errors in your user CLIs (invalid ones are skipped at startup). Then run it: `ccbox --cli <name>` — fix and rerun; nothing needs a rebuild.

The name is the directory name (lowercase). A user CLI overrides a same-name built-in.
