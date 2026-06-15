# Lessons

Training data from past failures — each entry the densest correction that retrains the context. Lessons are injected into context every session because their triggers are diffuse or invisible to Claude. If you can write a crisp when-to-apply hook for it, it's a memory (recall-gated), not a lesson.

## Format

One file, a flat list of atomic entries. Each entry is an imperative rule, a dated general case, and an example. The example is a compressed instance that grounds the rule — reach for a diff when one shows it better than prose, omit it otherwise.

````markdown
### imperative rule
[YYYY-MM-DD]: the general case — when it applies and what to do, stated so it transfers.
e.g. (compressed instance of the general case, may optionally include a diff formatted as below or be only just a diff)
`path/name-[YYYY-MM-DD].ext`:
```diff
a minimal wrong→right
```
````

The general case must transfer past its example — if the `[date]:` line just narrates the diff in words, it earns nothing. The leading `[date]` is when the lesson was born; the filename `-[date]` is the snapshot the example was cut from and is 100% a stale marker: trust the diff for the lesson, not for the file's present state.

Maintenance — before appending, check whether an entry already covers it and sharpen that one instead. Remove a lesson once it's reflex or a newer entry subsumes it. Append only if the failure would plausibly recur; never retire for age alone.

## Lessons

### Blank line marks an intent-group boundary, not every sibling
[2026-06-15]: In a list of like items, blank lines separate groups that differ in intent — blank-separating every sibling dissolves the grouping signal. Space by intent, not by item.
e.g. `.github/workflows/ci-[2026-06-15].yml`:
```diff
   - uses: actions/checkout@v4
-
   - name: lint
-
   - run: make lint
```

### Earn the line — say only what the name/code/structure doesn't already
[2026-06-15]: A comment or description that restates what its name, code, or structure already conveys is noise; keep only the bit the reader can't infer. Holds for code comments and doc prose alike.
e.g. a CLAUDE.md entry where `tests/` already implies its role, or a CI comment re-listing the steps it labels — `.github/workflows/ci-[2026-06-15].yml`:
```diff
-# Host-only checks (no Docker): hadolint, shellcheck, ruby allowlist test
+# Host-only checks (no Docker)
```

### Assert file contents against the file I Read, not a fetched lookalike
[2026-06-15]: Before claiming a file contains or lacks something, ground it in the bytes I actually Read — never in WebFetch'd reference content or memory of a similar file. Quote the specific line.
e.g. claimed the Dockerfile lacked env vars that were sitting on lines 17–18, having merged fetched reference docs with the user's real file.

### When the user repeats a correction, stop explaining and run the one check that settles it
[2026-06-15]: A repeated objection means my model is wrong, not that I was unclear — stop re-explaining my framing and run the cheapest command whose output is dispositive. Watch for bending the user's terms back to whatever I was just working on.
e.g. user kept asking about `bats` while I kept answering about `node`; `command -v bats` (empty) + `echo $PATH` (no shims dir) ended it in two lines.
