package proxy

import (
	"context"
	"net/http"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndRun_ProbeFails(t *testing.T) {
	// no proxy in the middle: the clients dial straight, so a down server reads
	// as the allowed host being unreachable — the egress wall unproven
	origProxy := proxyFrom
	proxyFrom = http.ProxyURL(nil)
	t.Cleanup(func() { proxyFrom = origProxy })
	origAllowed, origBlocked, origDirect := allowedURL, blockedURL, directURL
	allowedURL, blockedURL, directURL = downServer(t), downServer(t), downServer(t)
	t.Cleanup(func() { allowedURL, blockedURL, directURL = origAllowed, origBlocked, origDirect })

	// the share pin is the other probe's subject: aim it at a stub so this test
	// stays on the policy
	probeShareRO = func() error { return syscall.EROFS }
	openAllow = func() error { return syscall.EROFS }
	t.Cleanup(func() { probeShareRO = mkdirInShare })
	t.Cleanup(func() { openAllow = openAllowFile })

	var worked atomic.Bool
	err := ValidateAndRun(t.Context(), func(context.Context) error {
		worked.Store(true)
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowed host unreachable")
	assert.False(t, worked.Load(), "work never runs unless the egress wall is proven")
}
