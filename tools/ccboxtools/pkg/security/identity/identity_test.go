package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/proc"
)

func TestVerify_NoProxyArrival(t *testing.T) {
	arrived := func(mutate func(*proc.Status)) *proc.Status {
		st := &proc.Status{UID: [4]int{1000, 1000, 1000, 1000}, NoNewPrivs: true}
		if mutate != nil {
			mutate(st)
		}
		return st
	}

	tests := []struct {
		name    string
		st      *proc.Status
		wantErr string
	}{
		{"arrived", arrived(nil), ""},
		{
			"no-new-privs missing",
			arrived(func(st *proc.Status) { st.NoNewPrivs = false }),
			"no_new_privs is not set",
		},
		{
			"caps survive",
			arrived(func(st *proc.Status) { st.CapBnd = proc.CapSet{Lo: 1 << 8} }),
			"CapBnd must be empty",
		},
		{
			"uid mismatch",
			arrived(func(st *proc.Status) { st.UID = [4]int{0, 1000, 1000, 1000} }),
			"want uid 1000 on every slot",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readStatus = func() (*proc.Status, error) { return tt.st, nil }
			t.Cleanup(func() { readStatus = proc.ReadStatus })

			err := Verify(Opts{})

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
	for name, isExec := range map[string]bool{"exec": true, "run": false} {
		t.Run(name, func(t *testing.T) {
			err := Verify(Opts{IsProxy: true, IsExec: isExec})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "requires root")
		})
	}
}

func TestExecCaps_FundsTheWallSession(t *testing.T) {
	for _, c := range []int{capSetGID, capSetUID, capSetPCAP, capSysAdmin} {
		assert.True(t, proc.CapSet{Lo: execCaps}.Has(1<<c), "execCaps lacks cap %d", c)
	}
	assert.False(t, proc.CapSet{Lo: execCaps}.Has(1<<capNetAdmin),
		"a session crosses the netns but builds nothing: execCaps carries no NET_ADMIN")
	assert.False(t, proc.CapSet{Lo: execCaps}.Has(1<<capNetBindService),
		"the budgets never carry capNetBindService: the egress wall's ports need no privilege")
}

func TestRunCaps_AddsTheWallBoot(t *testing.T) {
	// runCaps is execCaps plus NET_ADMIN — the veth build, the boot's alone
	assert.Equal(t, proc.CapSet{Lo: execCaps | 1<<capNetAdmin}, proc.CapSet{Lo: runCaps})
}
