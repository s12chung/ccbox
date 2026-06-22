#!/usr/bin/env bash
# Soft tripwire (hygiene, NOT a boundary): prompts when a Bash command names a
# secret path, catching the lazy read the Read-deny rules miss (cat blocked, but
# `python -c "open('.env')"` is just `python` to the permission layer).
#
# Matched on the command STRING, so a rename, base64'd path, or generated script
# slips past. The real boundary is keeping secrets out of the mount + the egress
# wall. This only raises a prompt on the obvious path.
set -euo pipefail

cmd=$(jq -r '.tool_input.command // ""')

# Secret-path tokens — err toward asking (over-match is fine for a tripwire)
if printf '%s' "$cmd" | grep -Eq '\.env([^A-Za-z0-9_]|$)|\.env\.|\.envrc|(^|/)secrets/|\.pem([^A-Za-z0-9]|$)|\.key([^A-Za-z0-9]|$)|\.aws/|\.ssh/'; then
  jq -nc '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "ask",
      permissionDecisionReason: "Command references a secret path — confirm this read/use is intended."
    }
  }'
fi

# Exit 0 with no output = no decision; normal permission flow applies.
exit 0
