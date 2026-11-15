#!/usr/bin/env bats
# The host-daemon suite: the exec path's no-new-privileges proof, and the tcp
# port bindings' loopback reach into the workload.

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

# vnc_banner prints the desktop's handshake banner off the port the --vnc
# variant publishes by itself
vnc_banner() {
    { exec 3<>/dev/tcp/127.0.0.1/5900; } 2>/dev/null || return
    IFS= read -r banner <&3
    exec 3<&- 3>&-
    [ -n "$banner" ] && printf '%s\n' "$banner"
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

@test "port bindings: the host's loopback reaches the workload" {
    require_docker
    port=3987

    (
        stage_fail() { echo "== $1 failed — exit $2 ($3)"; exit "$2"; }
        # request 1 retries the boot out: until the server lands, a published
        # port may still close empty (curl 52) instead of refusing
        curl -s --retry 15 --retry-delay 1 --retry-all-errors "http://127.0.0.1:$port/" \
            || stage_fail "request 1" $? "refused = the port was never published; empty reply = the boot never served"
        vnc_banner || stage_fail "vnc banner" $? "5900 is published only by a --vnc variant"
        # the server is up once request 1 lands — no retry
        curl -s "http://localhost:$port/" || stage_fail "request 2" $? "the server stopped before answering twice"
    ) >"$BATS_TEST_TMPDIR/curl.out" 2>&1 & # a background write to the session's tty suspends it
    curl_pid=$!

    # the server stops after 2 requests or 2 seconds — the exec returns with
    # it, and the leave stops the devbox; the boot's chatter shows only when
    # the exec itself fails
    "${BIN:-dist/ccbox}" exec --vnc --port-bindings-tcp "$port" -- python3 tests/port-bindings-server.py "$port" \
        >"$BATS_TEST_TMPDIR/exec.out" 2>&1 || { cat "$BATS_TEST_TMPDIR/exec.out"; return 1; }

    curl_status=0
    wait "$curl_pid" || curl_status=$?
    out="$(<"$BATS_TEST_TMPDIR/curl.out")"
    if [ "$curl_status" -ne 0 ]; then
        echo "the request chain stopped at a stage — $out"
        return 1
    fi
    if [[ "$out" != *"RFB "* ]] || [[ "$out" != *"ccbox port bindings"* ]]; then
        echo "the chain's answers miss the desktop's banner or the server's body — $out"
        return 1
    fi
}
