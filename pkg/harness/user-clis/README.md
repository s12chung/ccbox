Drop-in CLIs live here as `<name>/CLI.yaml` (+ optional `<name>/config/`), the same format as the  built-ins — see https://github.com/s12chung/ccbox/tree/main/pkg/harness/clis for examples.

TLDR: create `<name>/CLI.yaml` here, copy a built-in's format, then run `ccbox --cli <name>` (or set `cli:` in `.ccbox.yaml`). Your CLI overrides a same-name built-in; an invalid CLI.yaml is skipped with a startup warning.

For a guided walkthrough, see [SKILL.md](SKILL.md).
