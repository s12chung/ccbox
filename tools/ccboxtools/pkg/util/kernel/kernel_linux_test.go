//go:build linux

package kernel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostureProbes_DenyUnderTheLiveProfile(t *testing.T) {
	denied, err := UnshareUserNS(t.Context())
	if err != nil || !denied {
		t.Skipf("the live profile does not hold the userns door (denied=%v, err=%v)", denied, err)
	}

	for _, probe := range []struct {
		name   string
		denied func() (bool, error)
	}{
		{"clone", CloneUserNS},
		{"clone3", Clone3UserNS},
		{"io_uring_setup", IoUringSetup},
	} {
		denied, err := probe.denied()
		require.NoError(t, err, probe.name)
		assert.True(t, denied, "%s must be denied under the live profile", probe.name)
	}
}
