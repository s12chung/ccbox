#!/usr/bin/env bash
# Fail-closed identity + egress verification. Runs at container start. The egress checks
# apply only when the proxy env vars are live (the only place the wall actually exists) —
# `ccbox --no-proxy` runs on plain bridge networking without them. If any check fails,
# the container refuses to start.
set -euo pipefail

fail() { echo "security-entrypoint: FAIL — $1" >&2; exit 1; }

# Identity: must run as the unprivileged ccbox user (uid 1000), never root, no sudo.
[ "$(id -u)" -eq 1000 ] || fail "not uid 1000 (got $(id -u))"
[ "$(id -un)" = ccbox ] || fail "user is not ccbox (got $(id -un))"
if command -v sudo >/dev/null 2>&1; then fail "sudo is present"; fi

# Git config: the host's global git dir binds here read-only
GIT_CONFIG_MOUNT=/home/ccbox/.config/git
if [ -d "$GIT_CONFIG_MOUNT" ]; then
  # mountinfo field 6 is the per-mount options; last match wins as the effective mount.
  opts=$(awk -v d="$GIT_CONFIG_MOUNT" '$5 == d { o = $6 } END { print o }' /proc/self/mountinfo)
  case ",$opts," in
    *,ro,*) ;;
    *) fail "$GIT_CONFIG_MOUNT is not a read-only mount (opts: ${opts:-none})" ;;
  esac
fi

# Wall probes, only when the proxy env is live (no proxy env = no wall to verify).
if [ -n "${http_proxy:-}" ]; then
  # api.github.com returns 200 unauthenticated and matches the proxy allowlist,
  # so it makes a clean positive probe. Overridable for other allowlists.
  ALLOWED_URL="${WALL_ALLOWED_URL:-https://api.github.com}"
  BLOCKED_URL="${WALL_BLOCKED_URL:-https://example.com}"
  DIRECT_URL="${WALL_DIRECT_URL:-https://1.1.1.1}"

  # 1. Positive: an allowlisted host must be reachable through the proxy.
  curl -fsS --max-time 5 -o /dev/null "$ALLOWED_URL" \
    || fail "allowed host unreachable ($ALLOWED_URL) — proxy or network is down"

  # 2. Negative (proxy filter): a non-allowlisted host must be rejected by tinyproxy.
  #    No -S: curl failing here is the success case and must stay silent.
  if curl -fs --max-time 5 -o /dev/null "$BLOCKED_URL"; then
    fail "blocked host reachable through proxy ($BLOCKED_URL) — allowlist not enforced"
  fi

  # 3. Negative (isolation): bypassing the proxy must have no route at all.
  #    Catches tools that ignore http_proxy/https_proxy; proves --internal holds.
  #    No -S: curl failing here is the success case and must stay silent.
  if curl -fs --max-time 5 -o /dev/null --noproxy '*' "$DIRECT_URL"; then
    fail "direct egress works ($DIRECT_URL) — network is not --internal"
  fi
fi

if [ -n "${CLI_PKGINFO:-}" ]; then
  ccboxtools update
fi

exec "$@"