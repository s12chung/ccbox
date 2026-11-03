package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

func TestExecMap_Cmd(t *testing.T) {
	config, err := cfg.Load(t.TempDir(), cfg.Config{CLIName: new("claude")}, false)
	require.NoError(t, err)
	em := NewExecMap(config.CLI())

	assert.Equal(t, execArgv("claude"), em.cmd(ExecMode{}, false))
	assert.Equal(t, execArgv("bash"), em.cmd(ExecMode{Shell: true}, false), "bare exec drops into the image's shell")
	assert.Equal(t, execArgv("make", "test"),
		em.cmd(ExecMode{Shell: true, Args: []string{"make", "test"}}, false), "exec execs the args")
	assert.Equal(t, execArgv("claude", "-c"), em.cmd(ExecMode{Continue: true}, false))
	assert.Equal(t, execArgv("claude", "--resume", "sess"),
		em.cmd(ExecMode{Resume: true, Args: []string{"sess"}}, false))
	assert.Equal(t, []string{"ccboxtools", "exec", "--", "bash"}, em.cmd(ExecMode{Shell: true}, true),
		"a plain variant runs no egress wall: the session is the image's user, already arrived")
}

// execArgv is the session argv under the egress wall's drop chain
func execArgv(argv ...string) []string {
	return append(proxy.DropChain("ccboxtools", "proxy", "exec", "--"), argv...)
}
