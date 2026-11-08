// Package proc is the kernel's process mechanics: child reaping, fd duplication,
// the /proc status protocol, the arrival's proof, and capability masks.
package proc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// DupStream dups from onto to.
func DupStream(from, to int) error { return dup(from, to) }

// ReapOne waits out one exited child without blocking; pid <= 0 when none waits.
func ReapOne() (syscall.WaitStatus, int) {
	var ws syscall.WaitStatus
	pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
	if err != nil {
		return ws, -1
	}
	return ws, pid
}

// The status fields the drop's proof reads — the protocol with /proc/<pid>/status.
const (
	uidField        = "Uid"
	gidField        = "Gid"
	groupsField     = "Groups"
	capInhField     = "CapInh"
	capPrmField     = "CapPrm"
	capEffField     = "CapEff"
	capBndField     = "CapBnd"
	capAmbField     = "CapAmb"
	noNewPrivsField = "NoNewPrivs"
)

// Status is the kernel's account of a process's identity and capabilities,
// from /proc/<pid>/status.
type Status struct {
	UID        [4]int // real, effective, saved, fs
	GID        [4]int // real, effective, saved, fs
	Groups     []int  // supplementary — a kept group like shadow hands over its files
	CapInh     CapSet // inherited across exec
	CapPrm     CapSet // permitted — the pool CapEff raises from
	CapEff     CapSet // effective — what's exercised right now
	CapBnd     CapSet // bounding — the ceiling on what any exec can ever gain
	CapAmb     CapSet // ambient — inherited across exec for non-root
	NoNewPrivs bool
}

// VerifyDrop proves a status is a completed drop to uid:gid: the unprivileged
// proof, plus no-new-privs held.
func VerifyDrop(st *Status, uid, gid int) error {
	if err := VerifyUnprivileged(st, uid, gid); err != nil {
		return err
	}
	if !st.NoNewPrivs {
		return errors.New("no_new_privs is not set")
	}
	return nil
}

// VerifyUnprivileged proves a status is unprivileged to uid:gid: the identity
// on every uid and gid slot, no supplementary group beyond the primary, zero
// capabilities on every set.
func VerifyUnprivileged(st *Status, uid, gid int) error {
	if st.UID != [4]int{uid, uid, uid, uid} {
		return fmt.Errorf("want uid %d on every slot (got %v)", uid, st.UID)
	}
	if st.GID != [4]int{gid, gid, gid, gid} {
		return fmt.Errorf("want gid %d on every slot (got %v)", gid, st.GID)
	}
	// the engine seeds the primary gid into the supplementary set on launch —
	// beyond-primary is the line a kept group like shadow would cross
	for _, g := range st.Groups {
		if g != gid {
			return fmt.Errorf("supplementary group beyond gid %d: got %v", gid, st.Groups)
		}
	}
	sets := []struct {
		name string
		set  CapSet
	}{
		{capInhField, st.CapInh},
		{capPrmField, st.CapPrm},
		{capEffField, st.CapEff},
		{capBndField, st.CapBnd},
		{capAmbField, st.CapAmb},
	}
	for _, s := range sets {
		if !s.set.IsZero() {
			return fmt.Errorf("%s must be empty, got %s", s.name, s.set)
		}
	}
	return nil
}

// CapSet is a 64-bit capability mask, read as hex.
type CapSet struct{ Lo, Hi uint64 }

// IsZero reports whether every bit is clear.
func (c CapSet) IsZero() bool { return c.Lo == 0 && c.Hi == 0 }

// Has reports whether every bit of bits is set — the high word counts too: a
// cap above bit 63 means the kernel knows caps this budget doesn't.
func (c CapSet) Has(bits uint64) bool { return c.Hi == 0 && c.Lo&bits == bits }

// Equal reports whether the set is exactly bits — none short, none beyond.
func (c CapSet) Equal(bits uint64) bool { return c.Hi == 0 && c.Lo == bits }

func (c CapSet) String() string {
	if c.Hi == 0 {
		return fmt.Sprintf("%#x", c.Lo)
	}
	return fmt.Sprintf("%#x:%#x", c.Hi, c.Lo)
}

// ReadStatus parses this process's own status — the arrival's ground truth.
func ReadStatus() (*Status, error) {
	f, err := os.Open("/proc/self/status") //nolint:gosec // the fixed procfs path
	if err != nil {
		return nil, err
	}
	defer func() { log.WarnErr("close status", f.Close()) }()
	return parseStatus(f)
}

// parseStatus parses a /proc status body — the arrival's ground truth.
func parseStatus(r io.Reader) (*Status, error) {
	st := &Status{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		name, value, found := strings.Cut(sc.Text(), ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		seen[name] = true

		var err error
		switch name {
		case uidField:
			st.UID, err = parseIDFields(value)
		case gidField:
			st.GID, err = parseIDFields(value)
		case groupsField:
			st.Groups, err = parseGroups(value)
		case capInhField:
			st.CapInh, err = parseCapSet(value)
		case capPrmField:
			st.CapPrm, err = parseCapSet(value)
		case capEffField:
			st.CapEff, err = parseCapSet(value)
		case capBndField:
			st.CapBnd, err = parseCapSet(value)
		case capAmbField:
			st.CapAmb, err = parseCapSet(value)
		case noNewPrivsField:
			st.NoNewPrivs = value == "1"
		}
		if err != nil {
			return nil, fmt.Errorf("status %s: %w", name, err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// a missing field fails closed: the drop's proof is only as good as the
	// kernel actually reporting each of them — the Groups line needs kernel 4.16
	for _, name := range []string{uidField, gidField, groupsField, capInhField, capPrmField, capEffField, capBndField, capAmbField, noNewPrivsField} {
		if !seen[name] {
			return nil, fmt.Errorf("status lacks %s", name)
		}
	}
	return st, nil
}

// parseIDFields reads a status id line: 4 space-separated slots, Uid and Gid
// alike.
func parseIDFields(s string) ([4]int, error) {
	fields := strings.Fields(s)
	if len(fields) != 4 {
		return [4]int{}, fmt.Errorf("want 4 id fields, got %q", s)
	}
	var ids [4]int
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return [4]int{}, err
		}
		ids[i] = n
	}
	return ids, nil
}

// parseGroups reads the supplementary groups — an empty value is no groups,
// which a completed --clear-groups drop always produces.
func parseGroups(s string) ([]int, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, nil
	}
	groups := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, err
		}
		groups = append(groups, n)
	}
	return groups, nil
}

// parseCapSet reads a status cap mask: 16 hex digits, or two comma-separated
// words once the kernel outgrows 64 bits.
func parseCapSet(s string) (CapSet, error) {
	hex := strings.ReplaceAll(s, ",", "")
	if len(hex) > 16 {
		hi, err := strconv.ParseUint(hex[:len(hex)-16], 16, 64)
		if err != nil {
			return CapSet{}, err
		}
		lo, err := strconv.ParseUint(hex[len(hex)-16:], 16, 64)
		if err != nil {
			return CapSet{}, err
		}
		return CapSet{Lo: lo, Hi: hi}, nil
	}
	lo, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return CapSet{}, err
	}
	return CapSet{Lo: lo}, nil
}
