package cmd

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/util/must"
)

func TestMain(m *testing.M) {
	// ignore any user clis on this machine: tests pin the embedded set
	dir := must.Get(os.MkdirTemp("", "ccbox-cmd-test"))
	clitmpl.SetUserConfigDir(dir)
	cli.Load()
	code := m.Run()
	must.Do(os.RemoveAll(dir))
	os.Exit(code)
}
