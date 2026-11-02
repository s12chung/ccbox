#!/usr/bin/env bats
# The exec path's no-new-privileges proof.

load test_helper

image="s12chung/ccbox:latest"

nnp_name=""

teardown() {
    if [ -n "$nnp_name" ]; then
        docker rm -f "$nnp_name" >/dev/null 2>&1 || true
    fi
}

boot_with_nnp() {
    nnp_name="ccbox-test-nnp-$1"
    # the real entrypoint rejects no-new-privileges=false — sleep keeps the
    # container up so `ccbox exec` can do its thing
    docker run -d --name "$nnp_name" --cap-drop ALL \
        --security-opt "no-new-privileges=$1" \
        --entrypoint sleep "$image" infinity >/dev/null
}

@test "no-new-privileges: first exec refuses on a boot without it" {
    require_docker
    boot_with_nnp false

    run docker exec "$nnp_name" ccboxtools exec -- true
    [ "$status" -ne 0 ]
    [[ "$output" == *"no_new_privs is not set"* ]]
}

@test "no-new-privileges: first exec passes on a boot with it" {
    require_docker
    boot_with_nnp true

    run docker exec "$nnp_name" ccboxtools exec -- true
    [ "$status" -eq 0 ]
}
