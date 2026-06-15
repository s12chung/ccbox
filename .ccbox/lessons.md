# Lessons

Training data from past failures — each entry the densest correction that retrains the context.

Format — one file, atomic entries (one lesson each), two sections:
- **Judgment** — decisions and invisible-behavior gotchas a diff can't show. `### imperative rule` + one line of why. No diff.
- **Shape** — visible code/config shape. `### imperative rule` + a minimal wrong→right ` ```diff `; the diff shows the lesson, so add prose only for a non-obvious why. Counter-examples (a revert of mine) still read wrong→right.

Pick the section by type: if it's "what to do / what bit me," it's Judgment; if the lesson is "how it looks," it's Shape.

## Judgment

### Assert file contents against the file I Read, not a fetched lookalike
> Claimed the Dockerfile lacked env vars that sat on lines 17–18 — I'd merged WebFetch'd reference content with the user's file. Quote the specific line when claiming presence/absence.

### `mise install --system` installs but writes no shims or config
> Installs to the shared dir yet places no shims/config → ruby/go/python/gem/pip missing from PATH at runtime, invisible until a test runs. Expose tools via a config: COPY `mise-system.toml` → `/etc/mise/config.toml`, then `MISE_DATA_DIR=/usr/local/share/mise mise install` (reads it, installs shared, auto-shims). `MISE_DATA_DIR` build-only so runtime `mise use` still targets `~/.local`.

## Shape

### Comment earns only the non-obvious bit, not a re-listing of the recipe
```diff
-# Host-only checks (no Docker): hadolint, shellcheck, ruby allowlist test
+# Host-only checks (no Docker)
 ci:
```

### Blank line marks an intent-group boundary, not every sibling
```diff
   - uses: actions/checkout@v4
-
   - name: lint
-
   - run: make test.local
```