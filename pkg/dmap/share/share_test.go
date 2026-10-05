package share

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/testutil"
	"github.com/s12chung/ccbox/pkg/cli"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.MainHome(m, cli.Load))
}
