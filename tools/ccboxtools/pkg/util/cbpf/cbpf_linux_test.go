//go:build linux

package cbpf

import (
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRenderDenyList_OnThisArch(t *testing.T) {
	prog, err := RenderDenyList([]string{"setns", "bpf"})

	require.NoError(t, err)
	// 4 program instructions plus one deny pair per name
	assert.Len(t, prog, 8*(4+2*2))
	assert.Equal(t, uint32(retAllow), binary.NativeEndian.Uint32(prog[len(prog)-4:]))
}

func TestRenderDenyList_RefusesUnknownNames(t *testing.T) {
	_, err := RenderDenyList([]string{"bpf", "nosuchsyscall"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nosuchsyscall")
}

func TestAuditArches_MatchTheKernelABI(t *testing.T) {
	// the local wire values must never drift from the kernel's
	arch, ok := auditArches[runtime.GOARCH]
	require.True(t, ok, "this build arch must have a render")

	kernel := map[string]uint32{
		"amd64":   unix.AUDIT_ARCH_X86_64,
		"arm64":   unix.AUDIT_ARCH_AARCH64,
		"ppc64le": unix.AUDIT_ARCH_PPC64LE,
		"riscv64": unix.AUDIT_ARCH_RISCV64,
		"s390x":   unix.AUDIT_ARCH_S390X,
	}
	assert.Equal(t, kernel[runtime.GOARCH], arch)
}

func TestSyscallNrs_Resolves(t *testing.T) {
	nrs, err := syscallNrs([]string{"setns", "bpf"})

	require.NoError(t, err)
	assert.Equal(t, []int{int(unix.SYS_SETNS), int(unix.SYS_BPF)}, nrs)
}
