#!/usr/bin/env bats
# The system mise and a project pinning to it must agree at runtime: tools
# resolved to the baked system install are marked (system) — nothing downloads.

bats_require_minimum_version 1.5.0

# the project fixture's dir, created in the test, cleaned up in teardown
proj_tmp=""

teardown() {
    rm -rf "$proj_tmp"
}

@test "project mise: workspace tools pinned to the system mise resolve (system)" {
    # the workspace mirrors the image's system config — with the lock beside it when
    # the image baked through one, exactly how ccbox overlays a project mise
    proj_tmp="$(mktemp -d)"
    cp /etc/mise/config.toml "$proj_tmp/mise.toml"
    if [ -f /etc/mise/mise.lock ]; then
        cp /etc/mise/mise.lock "$proj_tmp/"
    fi
    mise trust "$proj_tmp" >/dev/null

    run bash -c "cd '$proj_tmp' && mise ls --current 2>/dev/null"
    [ "$status" -eq 0 ]
    [[ "$output" == *"(system)"* ]]
    # every current tool resolves to its system install — none missing, none to download
    run ! grep -qv '(system)' <<<"$output"
}
