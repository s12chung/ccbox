// Package cbpf renders seccomp's classic-BPF programs — the raw native-endian
// sock_filter arrays setpriv's --seccomp-filter reads.
package cbpf

import (
	"encoding/binary"
	"errors"
	"runtime"
	"slices"
)

// The program's wire numbers, as consts so the package builds on every
// platform — classic BPF and seccomp's return verbs are frozen kernel ABI.
const (
	ldAbs = 0x20 // BPF_LD | BPF_W | BPF_ABS — a word at a seccomp_data offset
	jeq   = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	ret   = 0x06 // BPF_RET | BPF_K

	retErrno = 0x00050000 // SECCOMP_RET_ERRNO — the errno carried in the low bits
	retAllow = 0x7fff0000 // SECCOMP_RET_ALLOW

	errnoPerm = 1
)

// auditArches maps the build arch onto the seccomp reader's AUDIT_ARCH wire
// value.
var auditArches = map[string]uint32{
	"amd64":   0xc000003e,
	"arm64":   0xc00000b7,
	"ppc64le": 0xc0000015,
	"riscv64": 0xc00000f3,
	"s390x":   0x80000016,
}

// RenderDenyList renders an arch-checked deny list over the names' wire
// numbers — a second, stricter filter the kernel stacks beside the
// container's. A name without a wire number is an error, never a silent zero.
func RenderDenyList(names []string) ([]byte, error) {
	nrs, err := syscallNrs(names)
	if err != nil {
		return nil, err
	}
	arch, ok := auditArches[runtime.GOARCH]
	if !ok {
		return nil, errors.New("no AUDIT_ARCH for this build arch")
	}
	return renderDenyList(arch, nrs), nil
}

// renderDenyList encodes the program; sorted numbers keep the bytes stable.
func renderDenyList(auditArch uint32, nrs []int) []byte {
	slices.Sort(nrs)
	// seccomp_data: nr at 0, arch at 4; every match falls through to the EPERM
	// beside it
	prog := make([]byte, 0, 8*len(nrs)+32)
	inst := func(code uint16, jf uint8, k uint32) {
		prog = binary.NativeEndian.AppendUint16(prog, code)
		prog = append(prog, 0, jf)
		prog = binary.NativeEndian.AppendUint32(prog, k)
	}
	allowJump := uint8(2*len(nrs) + 1) //nolint:gosec // a deny list is nowhere near a jump-offset overflow
	inst(ldAbs, 0, 4)
	inst(jeq, allowJump, auditArch) // a foreign arch allows — the container's filter polices it
	inst(ldAbs, 0, 0)
	for _, nr := range nrs {
		inst(jeq, 1, uint32(nr)) //nolint:gosec // syscall numbers fit a word
		inst(ret, 0, retErrno|uint32(errnoPerm))
	}
	inst(ret, 0, retAllow)
	return prog
}
