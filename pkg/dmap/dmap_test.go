package dmap

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.MainHome(m, cli.Load))
}
