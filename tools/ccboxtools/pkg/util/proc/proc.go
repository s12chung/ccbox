// Package proc is the kernel's process mechanics: the /proc status protocol,
// capability masks, and the drop's proof.
package proc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The status fields the drop's proof reads — the protocol with /proc/<pid>/status.
const (
	uidField        = "Uid"
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
	CapInh     CapSet // inherited across exec
	CapPrm     CapSet // permitted — the pool CapEff raises from
	CapEff     CapSet // effective — what's exercised right now
	CapBnd     CapSet // bounding — the ceiling on what any exec can ever gain
	CapAmb     CapSet // ambient — inherited across exec for non-root
	NoNewPrivs bool
}

// VerifyDrop proves a status is a completed drop to uid: the unprivileged
// proof, plus no-new-privs held.
func VerifyDrop(st *Status, uid int) error {
	if err := VerifyUnprivileged(st, uid); err != nil {
		return err
	}
	if !st.NoNewPrivs {
		return errors.New("no_new_privs is not set")
	}
	return nil
}

// VerifyUnprivileged proves a status is unprivileged to uid: the identity on
// every uid slot, zero capabilities on every set.
func VerifyUnprivileged(st *Status, uid int) error {
	if st.UID != [4]int{uid, uid, uid, uid} {
		return fmt.Errorf("want uid %d on every slot (got %v)", uid, st.UID)
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

// parseStatus parses a /proc status body — the drop's ground truth.
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
			st.UID, err = parseUIDs(value)
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
	// kernel actually reporting each of them
	for _, name := range []string{uidField, capInhField, capPrmField, capEffField, capBndField, capAmbField, noNewPrivsField} {
		if !seen[name] {
			return nil, fmt.Errorf("status lacks %s", name)
		}
	}
	return st, nil
}

func parseUIDs(s string) ([4]int, error) {
	fields := strings.Fields(s)
	if len(fields) != 4 {
		return [4]int{}, fmt.Errorf("want 4 uid fields, got %q", s)
	}
	var uids [4]int
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return [4]int{}, err
		}
		uids[i] = n
	}
	return uids, nil
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
