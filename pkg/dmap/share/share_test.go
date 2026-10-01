package share

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/pkg/cli"
)

func TestMain(m *testing.M) {
	// AgentsMd resolves the CLI's config dir through the cli set
	cli.Load()
	os.Exit(m.Run())
}
