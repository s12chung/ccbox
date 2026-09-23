package cmd

import (
	"os"
	"testing"

	"github.com/s12chung/ccbox/pkg/harness"
)

func TestMain(m *testing.M) {
	harness.Load()
	os.Exit(m.Run())
}
