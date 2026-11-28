package dock

import (
	"context"
	"net"
	"testing"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCtxD_BackgroundedCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	d, err := client.NewClientWithOpts()
	require.NoError(t, err)
	ctxD := &CtxD{Ctx: ctx, D: d}

	bg := ctxD.BackgroundedCtx()
	cancel()
	assert.Same(t, d, bg.D)
	assert.NoError(t, bg.Ctx.Err()) // outlives the parent's cancel
}

func TestRefuseHeldPorts_Free(t *testing.T) {
	ctxD, err := NewCtxD(t.Context())
	require.NoError(t, err)

	li, port := loopbackPort(t)
	require.NoError(t, li.Close()) // free the port for the probe

	bindings := nat.PortMap{nat.Port(port + "/tcp"): []nat.PortBinding{{HostPort: port}}}
	assert.NoError(t, RefuseHeldPorts(ctxD, bindings))
}

func TestRefuseHeldPorts_Held(t *testing.T) {
	ctxD, err := NewCtxD(t.Context())
	require.NoError(t, err)

	li, port := loopbackPort(t)
	t.Cleanup(func() { require.NoError(t, li.Close()) })

	// a plain listener holds the port — the daemon finds no container to name, so
	// the error blames the port alone
	bindings := nat.PortMap{nat.Port(port + "/tcp"): []nat.PortBinding{{HostPort: port}}}
	err = RefuseHeldPorts(ctxD, bindings)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "127.0.0.1:"+port)
	assert.Contains(t, err.Error(), "already taken")
}

// loopbackPort opens a listener on a random loopback port, returning its port —
// the caller owns closing the listener
func loopbackPort(t *testing.T) (net.Listener, string) {
	t.Helper()
	li, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(li.Addr().String())
	require.NoError(t, err)
	return li, port
}
