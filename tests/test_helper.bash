# shellcheck shell=bash
# Shared helpers for the ccbox test suite. No external bats libraries.

# Assert that `command -v <cmd>` resolves to a path under <prefix>/.
assert_on_path_under() {
    local cmd="$1" prefix="$2" resolved
    resolved="$(command -v "$cmd")" || {
        echo "not on PATH: $cmd"
        return 1
    }
    case "$resolved" in
        "$prefix"/*) return 0 ;;
        *)
            echo "expected '$cmd' under '$prefix', got '$resolved'"
            return 1
            ;;
    esac
}
