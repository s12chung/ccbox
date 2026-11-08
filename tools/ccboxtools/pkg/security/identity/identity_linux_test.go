//go:build linux

package identity

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcWireConsts_MatchSyscall(t *testing.T) {
	assert.Equal(t, msRemount, uintptr(syscall.MS_REMOUNT))
	assert.Equal(t, msNoSuid, uintptr(syscall.MS_NOSUID))
	assert.Equal(t, msNoDev, uintptr(syscall.MS_NODEV))
	assert.Equal(t, msNoExec, uintptr(syscall.MS_NOEXEC))
}
