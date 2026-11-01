// Package identity is the workload's unprivileged identity: the boot's start
// budget, the seccomp posture it demands, every start's arrival proof, and the
// one-way drop.
package identity

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"os/user"
	"slices"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/proc"
)

const (
	ccboxUID = 1000 // the unprivileged identity every start lands on, matching the image's useradd

	capKill     = 5
	capSetGID   = 6
	capSetUID   = 7
	capSetPCAP  = 8
	capNetAdmin = 12
	capSysAdmin = 21

	capNetBindService = 10 // never in the budget: the egress wall's ports need no privilege

	// runCaps is the boot's budget: the netns build and setns, setpriv's drop, and
	// KILL — the root parent signals its dropped children, tinyproxy's reload included
	runCaps uint64 = 1<<capKill | 1<<capSetGID | 1<<capSetUID | 1<<capSetPCAP | 1<<capNetAdmin | 1<<capSysAdmin

	// the procfs remount's Linux wire numbers, as consts so the host binary builds on
	// every platform — the MS_* flags are not in syscall outside linux
	msRemount uintptr = 32
	msNoSuid  uintptr = 2
	msNoDev   uintptr = 4
	msNoExec  uintptr = 8

	setprivBin = "setpriv" // runs the drop
)

// capNames names each budgeted cap bit, in the form docker's CapAdd takes.
var capNames = map[int]string{
	capKill: "KILL", capSetGID: "SETGID", capSetUID: "SETUID",
	capSetPCAP: "SETPCAP", capNetAdmin: "NET_ADMIN", capSysAdmin: "SYS_ADMIN",
}

// RunCapNames renders the budget as the cap names docker's CapAdd takes —
// the shared source the host's run spec aims at, so CapAdd cannot drift.
func RunCapNames() []string { return budgetCapNames(runCaps) }

// Drop wraps argv in the setpriv drop to user.
func Drop(user string, argv ...string) []string {
	return slices.Concat(
		[]string{
			// the drop runs in setpriv, not Go — credentials are per-thread, so Go
			// can't drop them reliably
			setprivBin,
			"--reuid=" + user, "--regid=" + user, "--clear-groups",
			"--inh-caps=-all", "--ambient-caps=-all",
			"--bounding-set=-all", // no exec can ever regain a cap
			"--no-new-privs", "--",
		},
		argv,
	)
}

// budgetCapNames renders a budget's bits as cap names.
func budgetCapNames(caps uint64) []string {
	names := make([]string, 0, bits.OnesCount64(caps))
	for bit := range bits.Len64(caps) {
		if caps&(1<<bit) == 0 {
			continue
		}
		name, ok := capNames[bit]
		if !ok {
			panic(fmt.Sprintf("identity: cap bit %d is budgeted but has no name", bit))
		}
		names = append(names, name)
	}
	return names
}

// Verify proves the start: the egress wall boot's root budget or the no-proxy's
// ccbox arrival
func Verify(ctx context.Context, isProxy bool) error {
	if isProxy {
		return verifyProxyRequirements(ctx)
	}
	if err := VerifyAndResetDrop(); err != nil {
		return fmt.Errorf("no-proxy must arrive as ccbox with no caps and no_new_privs: %w", err)
	}
	if err := verifyUserNSDenied(ctx); err != nil {
		return fmt.Errorf("no-proxy must arrive under the boot's seccomp profile: %w", err)
	}
	return nil
}

// mountProc is the procfs remount — a var so tests can stub it.
var mountProc = mountProcfs

// HardenProcfs remounts /proc with hidepid=2, hiding the boot's other users from
// the dropped workload — it must precede any workload, while SYS_ADMIN is held.
func HardenProcfs() error {
	const (
		target = "/proc"
		data   = "hidepid=2"
		// the image's procfs defaults, re-asserted so the remount sheds none
		flags = msRemount | msNoSuid | msNoDev | msNoExec
	)
	if err := mountProc(target, flags, data); err != nil {
		return fmt.Errorf("procfs hardening: remount %s %s: %w", target, data, err)
	}
	return nil
}

// verifyProxyRequirements verifies a privileged egress wall start: root with
// exactly the budget, and userNS denied.
func verifyProxyRequirements(ctx context.Context) error {
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		return fmt.Errorf("egress wall start requires root (got uid %d euid %d)", os.Getuid(), os.Geteuid())
	}
	st, err := proc.ReadStatus()
	if err != nil {
		return err
	}
	if !st.CapPrm.Equal(runCaps) || !st.CapEff.Equal(runCaps) || !st.CapBnd.Equal(runCaps) {
		return fmt.Errorf("caps mismatch: CapPrm %s CapEff %s CapBnd %s, want exactly %#x", st.CapPrm, st.CapEff, st.CapBnd, runCaps)
	}
	return verifyUserNSDenied(ctx)
}

// probeUnshare is the userNS proof's probe — a var so tests can stub it.
var probeUnshare = unshareUserNS

// verifyUserNSDenied proves the seccomp posture: the probe must land on the profile's
// EPERM — anything else refuses the start; CLONE_NEWUSER needs no cap to attempt.
func verifyUserNSDenied(ctx context.Context) error {
	denied, err := probeUnshare(ctx)
	switch {
	case err != nil:
		return fmt.Errorf("verify userNS denied: %w", err)
	case !denied:
		return errors.New("unprivileged user namespaces are permitted — the seccomp profile must deny CLONE_NEWUSER")
	}
	return nil
}

// readStatus is a var so tests can stub it.
var readStatus = proc.ReadStatus

// VerifyAndResetDrop proves the drop and resets HOME to it
func VerifyAndResetDrop() error {
	st, err := readStatus()
	if err != nil {
		return err
	}
	if err := proc.VerifyDrop(st, ccboxUID); err != nil {
		return err
	}
	return resetHomeCache()
}

func resetHomeCache() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	if u.HomeDir != "" {
		return os.Setenv("HOME", u.HomeDir)
	}
	return nil
}
