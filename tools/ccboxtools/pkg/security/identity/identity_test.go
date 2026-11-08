package identity

import (
	"context"
	"errors"
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/kernel"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/proc"
)

func TestRunCaps_FundsTheWholeChain(t *testing.T) {
	// the build, the setns crossing, setpriv's drops, and the supervise signals — one budget
	for _, c := range []int{capKill, capSetGID, capSetUID, capSetPCAP, capNetAdmin, capSysAdmin} {
		assert.True(t, proc.CapSet{Lo: runCaps}.Has(1<<c), "runCaps lacks cap %d", c)
	}
	assert.False(t, proc.CapSet{Lo: runCaps}.Has(1<<capNetBindService),
		"the budget never carries capNetBindService: the egress wall's ports need no privilege")
}

func TestRunCapNames_RendersTheBudget(t *testing.T) {
	// the names docker's CapAdd takes, in bit order
	assert.Equal(t,
		[]string{"KILL", "SETGID", "SETUID", "SETPCAP", "NET_ADMIN", "SYS_ADMIN"},
		RunCapNames())
}

func TestDrop(t *testing.T) {
	// the full drop, every row load-bearing: the bounding set closes regaining,
	// the seccomp filter restores the cap-gated denials, no_new_privs closes
	// setuid/file-cap grants
	assert.Equal(t,
		[]string{
			"setpriv",
			"--reuid=tinyproxy", "--regid=tinyproxy", "--clear-groups",
			"--inh-caps=-all", "--ambient-caps=-all", "--bounding-set=-all",
			"--seccomp-filter=" + DropFilterPath,
			"--no-new-privs", "--",
			"sh",
		},
		Drop("tinyproxy", "sh"),
	)
}

func TestBudgetCapNames_RefusesUnnamedBit(t *testing.T) {
	assert.Panics(t, func() { budgetCapNames(1 << 3) },
		"a budgeted bit without a name would silently shrink the host's CapAdd")
}

func TestVerify_NoProxyArrival(t *testing.T) {
	arrived := func(mutate func(*proc.Status)) *proc.Status {
		st := &proc.Status{
			UID:        [4]int{1000, 1000, 1000, 1000},
			GID:        [4]int{1000, 1000, 1000, 1000},
			NoNewPrivs: true,
		}
		if mutate != nil {
			mutate(st)
		}
		return st
	}

	tests := []struct {
		name            string
		st              *proc.Status
		usernsPermitted bool
		wantErr         string
	}{
		{"arrived", arrived(nil), false, ""},
		{
			"no-new-privs missing",
			arrived(func(st *proc.Status) { st.NoNewPrivs = false }),
			false,
			"no_new_privs is not set",
		},
		{
			"caps survive",
			arrived(func(st *proc.Status) { st.CapBnd = proc.CapSet{Lo: 1 << 8} }),
			false,
			"CapBnd must be empty",
		},
		{
			"uid mismatch",
			arrived(func(st *proc.Status) { st.UID = [4]int{0, 1000, 1000, 1000} }),
			false,
			"want uid 1000 on every slot",
		},
		{
			"gid mismatch",
			arrived(func(st *proc.Status) { st.GID = [4]int{1000, 1000, 1000, 0} }),
			false,
			"want gid 1000 on every slot",
		},
		{
			"supplementary group survives",
			arrived(func(st *proc.Status) { st.Groups = []int{27} }),
			false,
			"supplementary group beyond gid 1000",
		},
		{
			"userns permitted",
			arrived(nil),
			true,
			"seccomp profile must deny",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readStatus = func() (*proc.Status, error) { return tt.st, nil }
			t.Cleanup(func() { readStatus = proc.ReadStatus })
			stubProbes(t, tt.usernsPermitted)

			err := Verify(t.Context(), false)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no-proxy must arrive")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestVerify_RefusesNonRootWallStart(t *testing.T) {
	// the test process is never root: an egress wall start must refuse it
	err := Verify(t.Context(), true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires root")
}

func TestHardenProcfs(t *testing.T) {
	tests := []struct {
		name     string
		mountErr error
		wantErr  string
	}{
		{"remounted", nil, ""},
		{"refused", errors.New("boom"), "procfs hardening"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotTarget string
			var gotFlags uintptr
			var gotData string
			mountProc = func(target string, flags uintptr, data string) error {
				gotTarget, gotFlags, gotData = target, flags, data
				return tt.mountErr
			}
			t.Cleanup(func() { mountProc = kernel.Mount })

			err := HardenProcfs()

			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, "/proc", gotTarget)
				assert.Equal(t, "hidepid=2", gotData)
				for _, flag := range []uintptr{msRemount, msNoSuid, msNoDev, msNoExec} {
					assert.NotZero(t, gotFlags&flag)
				}
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Contains(t, err.Error(), "boom")
		})
	}
}

func TestVerifyUserNSDenied_Unshare(t *testing.T) {
	tests := []struct {
		name    string
		denied  bool
		err     error
		wantErr string
	}{
		{"denied", true, nil, ""},
		{"permitted", false, nil, "unprivileged user namespaces are permitted"},
		{"probe failed", false, errors.New("boom"), "verify unshare denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubProbes(t, false)
			orig := probeUnshare
			probeUnshare = func(context.Context) (bool, error) { return tt.denied, tt.err }
			t.Cleanup(func() { probeUnshare = orig })

			err := verifyUserNSDenied(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestVerifyUserNSDenied_RawProbes(t *testing.T) {
	perProbe := func(name, wantErr string, fn func() (bool, error)) func(*testing.T) {
		return func(t *testing.T) {
			stubProbes(t, false)
			orig := postureProbes
			postureProbes = []postureProbe{{name, fn}}
			t.Cleanup(func() { postureProbes = orig })

			err := verifyUserNSDenied(t.Context())

			if wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), wantErr)
		}
	}
	for _, probe := range postureProbes {
		t.Run(probe.name+" denied", perProbe(probe.name, "", func() (bool, error) { return true, nil }))
		t.Run(probe.name+" permitted", perProbe(probe.name, probe.name+" is permitted", func() (bool, error) { return false, nil }))
		t.Run(probe.name+" failed", perProbe(probe.name, "verify "+probe.name+" denied", func() (bool, error) { return false, errors.New("boom") }))
	}
}

// stubProbes stubs every posture probe: the userns ones to usernsPermitted,
// io_uring denied, all reporting no error.
func stubProbes(t *testing.T, usernsPermitted bool) {
	t.Helper()
	origUnshare := probeUnshare
	probeUnshare = func(context.Context) (bool, error) { return !usernsPermitted, nil }
	t.Cleanup(func() { probeUnshare = origUnshare })
	origProbes := postureProbes
	postureProbes = []postureProbe{
		{"clone", func() (bool, error) { return !usernsPermitted, nil }},
		{"clone3", func() (bool, error) { return !usernsPermitted, nil }},
		{"io_uring_setup", func() (bool, error) { return true, nil }},
	}
	t.Cleanup(func() { postureProbes = origProbes })
}

func TestVerifyAndResetDrop_ResetsHome(t *testing.T) {
	// the drop re-pins HOME: docker set it for the arrival's original user — root
	// on an egress wall boot — and setpriv changes the uid but not the env
	t.Setenv("HOME", "/root")
	readStatus = func() (*proc.Status, error) {
		return &proc.Status{
			UID:        [4]int{1000, 1000, 1000, 1000},
			GID:        [4]int{1000, 1000, 1000, 1000},
			NoNewPrivs: true,
		}, nil
	}
	t.Cleanup(func() { readStatus = proc.ReadStatus })

	require.NoError(t, VerifyAndResetDrop())

	u, err := user.Current()
	require.NoError(t, err)
	assert.Equal(t, u.HomeDir, os.Getenv("HOME"))
}

func TestVerifyAndResetDrop_RefusesFailedDrop(t *testing.T) {
	// a refused arrival mutates nothing: the reset only follows a proven drop
	t.Setenv("HOME", "/root")
	readStatus = func() (*proc.Status, error) {
		return &proc.Status{UID: [4]int{0, 0, 0, 0}, NoNewPrivs: false}, nil
	}
	t.Cleanup(func() { readStatus = proc.ReadStatus })

	err := VerifyAndResetDrop()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "want uid 1000")
	assert.Equal(t, "/root", os.Getenv("HOME"))
}
