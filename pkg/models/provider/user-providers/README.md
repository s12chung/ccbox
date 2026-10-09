Drop-in providers live here as `<name>.yaml` — `domains:` is the only key, the name comes from the filename. The built-ins live at https://github.com/s12chung/ccbox/tree/main/pkg/models/provider/provider.go.

TLDR: create `<name>.yaml` here with your provider's API domains, then allowlist `ccbox-<name>-provider` (or `ccbox-all-providers`) in `allowlist:`. Your provider overrides a same-name built-in; an invalid provider yaml is skipped with a startup warning — run `ccbox doctor providers` to check them.
