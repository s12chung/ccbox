// Package identity is the workload's unprivileged identity: the egress wall's
// privileged starts and their one-way drop, and the no-proxy's unprivileged
// arrival proof — from the kernel's own account, before any untrusted code
// runs.
package identity

import (
	"fmt"
	"os"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/proc"
)

const (
	ccboxUID = 1000 // the unprivileged identity every start lands on, matching the image's useradd

	capSetGID   = 6
	capSetUID   = 7
	capSetPCAP  = 8
	capNetAdmin = 12
	capSysAdmin = 21

	capNetBindService = 10 // never in the budgets: the egress wall's ports need no privilege

	execCaps uint64 = 1<<capSetGID | 1<<capSetUID | 1<<capSetPCAP | 1<<capSysAdmin // the session's: enter the netns, drop to ccbox
	runCaps         = execCaps | 1<<capNetAdmin                                    // the run's: the session's, plus the network build
)

// Opts is Verify's start to prove.
type Opts struct {
	IsProxy bool
	IsExec  bool
}

// Verify proves the start the opts describe: the egress wall's root budget or
// the no-proxy's ccbox arrival.
func Verify(opts Opts) error {
	if opts.IsProxy {
		return verifyProxyRequirements(opts.IsExec)
	}
	if err := VerifyDropped(); err != nil {
		return fmt.Errorf("no-proxy must arrive as ccbox with no caps and no_new_privs: %w", err)
	}
	return nil
}

// verifyProxyRequirements verifies a privileged start of the egress wall:
// root with exactly the start's budget, or refuse.
func verifyProxyRequirements(isExec bool) error {
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		return fmt.Errorf("egress wall start requires root (got uid %d euid %d)", os.Getuid(), os.Geteuid())
	}
	st, err := proc.ReadStatus()
	if err != nil {
		return err
	}

	budget := execCaps
	if !isExec {
		budget = runCaps
	}
	if !st.CapEff.Equal(budget) || !st.CapBnd.Equal(budget) {
		return fmt.Errorf("caps mismatch: CapEff %s CapBnd %s, want exactly %#x", st.CapEff, st.CapBnd, budget)
	}
	return nil
}

// Drop is the one-way door's turn: give up the container's every privilege,
// landing on the workload identity.
func Drop() error {
	if err := proc.DropTo(ccboxUID); err != nil {
		return err
	}
	return VerifyDropped()
}

// readStatus is a var so tests aim it at a stub.
var readStatus = proc.ReadStatus

// VerifyDropped proves the dropped state, from the kernel's own account.
func VerifyDropped() error {
	st, err := readStatus()
	if err != nil {
		return err
	}
	return proc.VerifyDrop(st, ccboxUID)
}
