#!/usr/bin/env bash
# Soft tripwire (hygiene, NOT a boundary): prompts when a Bash command names a
# protected path, catching what the Read/Write/Edit deny rules miss (cat blocked,
# but `python -c "open('.env')"` is just `python` to the permission layer).
#
# Matched on the command STRING, so a rename, base64'd path, or generated script
# slips past. The real boundary is keeping secrets out of the mount + the egress
# wall. This only raises a prompt on the obvious path.
set -euo pipefail

cmd=$(jq -r '.tool_input.command // ""')

ask() {
  jq -nc --arg reason "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "ask",
      permissionDecisionReason: $reason
    }
  }'
  exit 0
}

# Secret-path reads — err toward asking (over-match is fine for a tripwire)
secret_files=(
  '\.env([^A-Za-z0-9_]|$)'
  '\.env\.'
  '\.envrc'
  '\.pem([^A-Za-z0-9]|$)'
  '\.key([^A-Za-z0-9]|$)'
)
secret_dirs=(
  '(^|/)secrets/'
  '\.aws/'
  '\.ssh/'
)
pattern=$(IFS='|'; printf '%s' "${secret_files[*]}|${secret_dirs[*]}")

if printf '%s' "$cmd" | grep -Eq "$pattern"; then
  ask "Command references a secret path — confirm this read/use is intended."
fi

# Exit 0 with no output = no decision; normal permission flow applies.
exit 0
