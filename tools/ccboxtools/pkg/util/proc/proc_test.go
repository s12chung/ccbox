package proc

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReapOne(t *testing.T) {
	cmd := exec.Command("true") //nolint:gosec,noctx // the reaper's throwaway child
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid

	var reaped int
	require.Eventually(t, func() bool {
		_, reaped = ReapOne()
		return reaped == pid
	}, 5*time.Second, 10*time.Millisecond, "reaps the exited child")
}

func TestVerifyDrop(t *testing.T) {
	dropped := func(mutate func(*Status)) *Status {
		st := &Status{
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
		name    string
		st      *Status
		wantErr string
	}{
		{"dropped", dropped(nil), ""},
		{
			"uid mismatch",
			dropped(func(st *Status) { st.UID = [4]int{0, 1000, 1000, 1000} }),
			"want uid 1000 on every slot",
		},
		{
			"gid mismatch",
			dropped(func(st *Status) { st.GID = [4]int{0, 1000, 1000, 1000} }),
			"want gid 1000 on every slot",
		},
		{
			"supplementary group survives",
			dropped(func(st *Status) { st.Groups = []int{42} }),
			"supplementary group beyond gid 1000",
		},
		{
			"caps survive",
			dropped(func(st *Status) { st.CapBnd = CapSet{Lo: 1 << 8} }),
			"CapBnd must be empty",
		},
		{
			"no-new-privs missing",
			dropped(func(st *Status) { st.NoNewPrivs = false }),
			"no_new_privs is not set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyDrop(tt.st, 1000, 1000)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestVerifyUnprivileged(t *testing.T) {
	tests := []struct {
		name    string
		st      *Status
		wantErr string
	}{
		{"unprivileged", &Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}}, ""},
		{
			"uid mismatch",
			&Status{UID: [4]int{0, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}},
			"want uid 1000 on every slot",
		},
		{
			"gid mismatch",
			&Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 0}},
			"want gid 1000 on every slot",
		},
		{
			"primary gid in the supplementary set",
			&Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}, Groups: []int{1000}},
			"",
		},
		{
			"supplementary group survives",
			&Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}, Groups: []int{1000, 27}},
			"supplementary group beyond gid 1000",
		},
		{
			"caps survive",
			&Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}, CapBnd: CapSet{Lo: 1 << 8}},
			"CapBnd must be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyUnprivileged(tt.st, 1000, 1000)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCapSet_IsZero(t *testing.T) {
	assert.True(t, CapSet{}.IsZero())
	assert.False(t, CapSet{Lo: 0x2011c0}.IsZero())
}

func TestCapSet_Has(t *testing.T) {
	some := CapSet{Lo: 0x2011c0}

	tests := []struct {
		name string
		set  CapSet
		bits uint64
		want bool
	}{
		{"has all its bits", some, 0x2011c0, true},
		{"has one bit", some, 1 << 21, true},
		{"lacks a bit it never had", some, 1 << 10, false},
		{"high word means not has", CapSet{Lo: 0x2011c0, Hi: 1}, 0x2011c0, false},
		{"zero", CapSet{}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.set.Has(tt.bits))
		})
	}
}

func TestCapSet_Equal(t *testing.T) {
	some := CapSet{Lo: 0x2011c0}

	tests := []struct {
		name string
		set  CapSet
		bits uint64
		want bool
	}{
		{"equals the bits", some, 0x2011c0, true},
		{"carries bits beyond", some, 1 << 21, false},
		{"lacks a bit of the bits", some, 0x2011c0 | 1<<10, false},
		{"high word means not equal", CapSet{Lo: 0x2011c0, Hi: 1}, 0x2011c0, false},
		{"zero", CapSet{}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.set.Equal(tt.bits))
		})
	}
}

func TestReadStatus_MatchesOS(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping off Linux")
	}

	// the test runs as some real user; ReadStatus must resolve it, not fail closed
	st, err := ReadStatus()

	require.NoError(t, err)
	assert.Equal(t, os.Getuid(), st.UID[0])
	assert.Equal(t, os.Getgid(), st.GID[0])
}

func TestParseStatus(t *testing.T) {
	dropped := "Uid:\t1000\t1000\t1000\t1000\n" +
		"Gid:\t1000\t1000\t1000\t1000\n" +
		"Groups:\t\n" +
		"CapInh:\t0000000000000000\n" +
		"CapPrm:\t0000000000000000\n" +
		"CapEff:\t0000000000000000\n" +
		"CapBnd:\t0000000000000000\n" +
		"CapAmb:\t0000000000000000\n" +
		"NoNewPrivs:\t1\n"

	// the image's boot budget: setgid, setuid, setpcap, net_admin, sys_admin
	const bootCaps = 1<<6 | 1<<7 | 1<<8 | 1<<12 | 1<<21

	tests := []struct {
		name    string
		body    string
		want    Status
		wantErr string
	}{
		{
			name: "dropped",
			body: dropped,
			want: Status{UID: [4]int{1000, 1000, 1000, 1000}, GID: [4]int{1000, 1000, 1000, 1000}, NoNewPrivs: true},
		},
		{
			name: "boot root",
			body: "Uid:\t0\t0\t0\t0\n" +
				"Gid:\t0\t0\t0\t0\n" +
				"Groups:\t\n" +
				"CapInh:\t0000000000000000\n" +
				"CapPrm:\t00000000002011c0\n" +
				"CapEff:\t00000000002011c0\n" +
				"CapBnd:\t00000000002011c0\n" +
				"CapAmb:\t0000000000000000\n" +
				"NoNewPrivs:\t0\n",
			want: Status{CapPrm: CapSet{Lo: bootCaps}, CapEff: CapSet{Lo: bootCaps}, CapBnd: CapSet{Lo: bootCaps}},
		},
		{
			name: "supplementary groups",
			body: strings.Replace(dropped, "Groups:\t\n", "Groups:\t1000 27\n", 1),
			want: Status{
				UID:        [4]int{1000, 1000, 1000, 1000},
				GID:        [4]int{1000, 1000, 1000, 1000},
				Groups:     []int{1000, 27},
				NoNewPrivs: true,
			},
		},
		{
			name: "high word",
			body: "Uid:\t1000\t1000\t1000\t1000\n" +
				"Gid:\t1000\t1000\t1000\t1000\n" +
				"Groups:\t\n" +
				"CapInh:\t0000000000000000\n" +
				"CapPrm:\t00000001,0000000000000000\n" +
				"CapEff:\t00000001,0000000000000000\n" +
				"CapBnd:\t00000001,0000000000000000\n" +
				"CapAmb:\t0000000000000000\n" +
				"NoNewPrivs:\t0\n",
			want: Status{
				UID:    [4]int{1000, 1000, 1000, 1000},
				GID:    [4]int{1000, 1000, 1000, 1000},
				CapPrm: CapSet{Hi: 1},
				CapEff: CapSet{Hi: 1},
				CapBnd: CapSet{Hi: 1},
			},
		},
		{
			name:    "missing field",
			body:    strings.Replace(dropped, "NoNewPrivs:\t1\n", "", 1),
			wantErr: "status lacks NoNewPrivs",
		},
		{
			name:    "missing gid",
			body:    strings.Replace(dropped, "Gid:\t1000\t1000\t1000\t1000\n", "", 1),
			wantErr: "status lacks Gid",
		},
		{
			name:    "missing groups",
			body:    strings.Replace(dropped, "Groups:\t\n", "", 1),
			wantErr: "status lacks Groups",
		},
		{
			name:    "bad gid fields",
			body:    strings.Replace(dropped, "Gid:\t1000\t1000\t1000\t1000", "Gid:\t1000", 1),
			wantErr: "want 4 id fields",
		},
		{
			name:    "bad uid fields",
			body:    strings.Replace(dropped, "Uid:\t1000\t1000\t1000\t1000", "Uid:\t1000", 1),
			wantErr: "want 4 id fields",
		},
		{
			name:    "bad groups",
			body:    strings.Replace(dropped, "Groups:\t\n", "Groups:\tccbox\n", 1),
			wantErr: "status Groups",
		},
		{
			name:    "bad cap hex",
			body:    strings.Replace(dropped, "CapEff:\t0000000000000000", "CapEff:\tzzz", 1),
			wantErr: "status CapEff",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := parseStatus(strings.NewReader(tt.body))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, *st)
		})
	}
}
