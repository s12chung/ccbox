package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

func TestExecMap_Cmd(t *testing.T) {
	config := loadConfig(t, "cli: claude\n")
	em := NewExecMap(config.CLI())

	assert.Equal(t, execArgv("claude"), em.cmd(ExecMode{}, true))
	assert.Equal(t, execArgv("bash"), em.cmd(ExecMode{Shell: true}, true), "bare exec drops into the image's shell")
	assert.Equal(t, execArgv("make", "test"),
		em.cmd(ExecMode{Shell: true, Args: []string{"make", "test"}}, true), "exec execs the args")
	assert.Equal(t, []string{"ccboxtools", "exec", "--", "bash"}, em.cmd(ExecMode{Shell: true}, false),
		"a plain variant runs no egress wall: the session is the image's user, already arrived")
}

// execArgv is the session argv under the egress wall's drop chain
func execArgv(argv ...string) []string {
	return append(proxy.DropChain("ccboxtools", "proxy", "exec", "--"), argv...)
}
