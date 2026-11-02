package seccomputil

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/moby/profiles/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootOpt_Whitelist(t *testing.T) {
	opt, err := BootOpt()

	require.NoError(t, err)
	require.True(t, strings.HasPrefix(opt, "seccomp="))
	var profile seccomp.Seccomp
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(opt, "seccomp=")), &profile))

	// the whitelist posture, over the engine's own arch map
	assert.Equal(t, specs.ActErrno, profile.DefaultAction)
	require.NotNil(t, profile.DefaultErrnoRet)
	assert.Equal(t, errnoPerm, *profile.DefaultErrnoRet)
	assert.NotEmpty(t, profile.ArchMap)

	// pins the merged profile to its golden: a profiles bump fails here for review
	rendered, err := json.MarshalIndent(&profile, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, golden(t, "TestBootOpt_Whitelist.json"), string(rendered)+"\n")
}

func TestBootProfile_KeepsTheBootLifelines(t *testing.T) {
	profile, err := bootProfile()
	require.NoError(t, err)

	// the boot builds the proxy under SYS_ADMIN: mount and setns stay
	// cap-allowed, and unshare's allow still admits NEWNET and NEWNS
	var lifelines *seccomp.Syscall
	for _, rule := range profile.Syscalls {
		if rule.Action == specs.ActAllow &&
			slices.Contains(rule.Names, "mount") &&
			slices.Contains(rule.Names, "setns") {
			lifelines = rule
			break
		}
	}
	require.NotNil(t, lifelines)
	require.NotNil(t, lifelines.Includes)
	assert.Equal(t, []string{"CAP_SYS_ADMIN"}, lifelines.Includes.Caps)
}

func TestBootProfile_ShutsTheUsernsDoor(t *testing.T) {
	profile, err := bootProfile()
	require.NoError(t, err)
	cloneRules := doorRules(t, profile, "clone")
	require.Len(t, cloneRules, 2)
	clone, cloneS390 := cloneRules[0], cloneRules[1]
	unshareRules := doorRules(t, profile, "unshare")
	require.Len(t, unshareRules, 1)
	unshare := unshareRules[0]

	// clone's flags: arg0 (s390's ABI: arg1); unshare's: arg0 on every ABI
	assert.Equal(t, []string{"clone"}, clone.Names)
	assert.Equal(t, s390Arches, clone.Excludes.Arches)
	assert.Nil(t, clone.Includes)
	assert.Equal(t, []string{"clone"}, cloneS390.Names)
	assert.Equal(t, s390Arches, cloneS390.Includes.Arches)
	assert.Nil(t, cloneS390.Excludes)
	assert.Equal(t, []string{"unshare"}, unshare.Names)
	assert.Nil(t, unshare.Includes)
	assert.Nil(t, unshare.Excludes)

	// the userns exception: a CLONE_NEWUSER call falls to the whitelist's EPERM
	for _, rule := range []*seccomp.Syscall{clone, cloneS390, unshare} {
		assert.Equal(t, specs.ActAllow, rule.Action)
		arg := rule.Args[0]
		assert.Equal(t, cloneNewUser, arg.Value)
		assert.Zero(t, arg.ValueTwo)
		assert.Equal(t, specs.OpMaskedEqual, arg.Op)
	}
	assert.Equal(t, uint(0), clone.Args[0].Index)
	assert.Equal(t, uint(0), unshare.Args[0].Index)
	assert.Equal(t, uint(1), cloneS390.Args[0].Index)
}

func TestBootProfile_DeniesClone3AndIoUring(t *testing.T) {
	profile, err := bootProfile()
	require.NoError(t, err)
	rules := denialRules(t, profile)
	clone3, uring := rules[0], rules[1]

	assert.Equal(t, []string{"clone3"}, clone3.Names)
	assert.Equal(t, []string{"io_uring_setup", "io_uring_enter", "io_uring_register"}, uring.Names)
	// ENOSYS, not EPERM — userspace must read the decoys as unsupported
	for _, rule := range []*seccomp.Syscall{clone3, uring} {
		assert.Equal(t, specs.ActErrno, rule.Action)
		require.NotNil(t, rule.ErrnoRet)
		assert.Equal(t, errnoNoSys, *rule.ErrnoRet)
	}
}

func TestBootProfile_DeniesPtraceFamily(t *testing.T) {
	profile, err := bootProfile()
	require.NoError(t, err)
	rules := denialRules(t, profile)
	ptraceFamily := rules[2]

	assert.Equal(t, []string{"ptrace", "process_vm_readv", "process_vm_writev", "pidfd_getfd", "process_madvise", "kcmp"}, ptraceFamily.Names)
	// EPERM, not ENOSYS — the calls exist; the profile forbids them
	assert.Equal(t, specs.ActErrno, ptraceFamily.Action)
	require.NotNil(t, ptraceFamily.ErrnoRet)
	assert.Equal(t, errnoPerm, *ptraceFamily.ErrnoRet)

	// the merge dropped the default profile's ptrace allows — the deny is each
	// name's only carrier
	for _, name := range ptraceFamily.Names {
		carriers := doorRules(t, profile, name)
		require.Len(t, carriers, 1)
		assert.Equal(t, ptraceFamily, carriers[0])
	}
}

func TestDefaultProfile_MatchesTheLibrary(t *testing.T) {
	// the module cache exists on every OS, so a missed regeneration fails
	// anywhere the tests run
	var library seccomp.Seccomp
	require.NoError(t, json.Unmarshal(libraryDefaultJSON(t), &library))

	var carried seccomp.Seccomp
	require.NoError(t, json.Unmarshal(seccompDefaultJSON, &carried))
	assert.Equal(t, &library, &carried)
}

// libraryDefaultJSON reads the library's default.json out of the module cache
func libraryDefaultJSON(t *testing.T) []byte {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", "github.com/moby/profiles/seccomp").Output()
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "default.json")) // #nosec G304 -- the library's own file
	require.NoError(t, err)
	return body
}

// denialRules returns the profile's denial side — the door's ENOSYS decoys
// plus the ptrace-family EPERM deny
func denialRules(t *testing.T, profile *seccomp.Seccomp) []*seccomp.Syscall {
	t.Helper()
	var rules []*seccomp.Syscall
	for _, rule := range profile.Syscalls {
		if rule.Action != specs.ActAllow {
			rules = append(rules, rule)
		}
	}
	require.Len(t, rules, 3)
	return rules
}

// doorRules returns the profile's rules carrying name
func doorRules(t *testing.T, profile *seccomp.Seccomp, name string) []*seccomp.Syscall {
	t.Helper()
	var rules []*seccomp.Syscall
	for _, rule := range profile.Syscalls {
		if slices.Contains(rule.Names, name) {
			rules = append(rules, rule)
		}
	}
	return rules
}

// golden reads a render's testdata fixture
func golden(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name)) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)
	return string(body)
}
