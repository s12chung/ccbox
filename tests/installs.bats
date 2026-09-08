#!/usr/bin/env bats
# Every ecosystem must let ccbox install into user space, on PATH, without root.
# Uses tiny throwaway packages; teardown removes them so reruns stay clean.

load test_helper

teardown() {
    npm uninstall -g cowsay >/dev/null 2>&1 || true
    gem uninstall -aIx lolcat >/dev/null 2>&1 || true
    pip uninstall -y pyfiglet >/dev/null 2>&1 || true
    rm -f "$HOME/go/bin/2fa"
    mise uninstall jq@1.7.1 >/dev/null 2>&1 || true
}

@test "login shell: /etc/profile.d restore survives /etc/profile's PATH reset" {
    # bats is a pure mise shim (no /usr/local/bin fallback), so it's the canary:
    # without the restore, /etc/profile drops the shims dir and this goes empty.
    run bash -lc 'command -v bats'
    [ "$status" -eq 0 ]

    run bash -lc 'echo "$PATH"'
    [[ "$output" == *"/usr/local/share/mise/shims"* ]]
    [[ "$output" == *"/opt/ccbox/clis/bin"* ]]
}

@test "coding cli: installed into the clis volume and runs" {
    local cli
    cli="$(jq -r .name <<<"$CLI_PKGINFO")"
    assert_on_path_under "$cli" "/opt/ccbox/clis/bin"

    run "$cli" --version
    [ "$status" -eq 0 ]
    [[ "$output" =~ [0-9]+\.[0-9]+\.[0-9]+ ]]
}

@test "npm: global install lands in ~/.npm-global and on PATH" {
    npm install -g cowsay --silent
    [ -x "$HOME/.npm-global/bin/cowsay" ]
    assert_on_path_under cowsay "$HOME/.npm-global/bin"
}

@test "gem: install lands in ~/.gem and on PATH (GEM_HOME)" {
    gem install lolcat --no-document --quiet
    ls -d "$HOME"/.gem/gems/lolcat-* >/dev/null
    assert_on_path_under lolcat "$HOME/.gem/bin"
}

@test "pip: install lands in ~/.local and on PATH (auto --user)" {
    pip install pyfiglet --quiet
    ls -d "$HOME"/.local/lib/python*/site-packages/pyfiglet >/dev/null
    assert_on_path_under pyfiglet "$HOME/.local/bin"
}

@test "go: install lands in ~/go/bin and on PATH (GOBIN)" {
    go install rsc.io/2fa@latest
    [ -x "$HOME/go/bin/2fa" ]
    assert_on_path_under 2fa "$HOME/go/bin"
}

@test "mise: user 'mise use' installs into ~/.local/share/mise" {
    local tmp
    tmp="$(mktemp -d)"
    (cd "$tmp" && mise use jq@1.7.1)
    [ -d "$HOME/.local/share/mise/installs/jq/1.7.1" ]
    run bash -c "cd '$tmp' && mise exec -- jq --version"
    [[ "$output" == *"1.7"* ]]
    rm -rf "$tmp"
}
