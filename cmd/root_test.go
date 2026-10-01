package cmd

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/pkg/cli"
)

func TestMain(m *testing.M) {
	cli.Load()
	os.Exit(m.Run())
}
