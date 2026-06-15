# Lessons

Training data from past failures — each entry the densest correction that retrains the context.

Format — one file, atomic entries (one lesson each), two sections:
- **Judgment** — decisions and invisible-behavior gotchas a diff can't show. `### imperative rule` + one line of why. No diff.
- **Shape** — visible code/config shape. `### imperative rule` + a minimal wrong→right ` ```diff `; the diff shows the lesson, so add prose only for a non-obvious why. Counter-examples (a revert of mine) still read wrong→right.

Pick the section by type: if it's "what to do / what bit me," it's Judgment; if the lesson is "how it looks," it's Shape.

## Judgment

### Assert file contents against the file I Read, not a fetched lookalike
> Claimed the Dockerfile lacked env vars that sat on lines 17–18 — I'd merged WebFetch'd reference content with the user's file. Quote the specific line when claiming presence/absence.

## Shape

### Blank line marks an intent-group boundary, not every sibling
```diff
   - uses: actions/checkout@v4
-
   - name: lint
-
   - run: make test.local
```