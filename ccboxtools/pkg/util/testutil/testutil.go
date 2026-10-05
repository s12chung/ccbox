// Package testutil centralizes the per-test env setup that both modules' tests share.
package testutil

import (
	"os"
	"testing"
)

// Home points $HOME at a fresh temp dir for the test's duration, returning it.
// XDG_CONFIG_HOME is cleared, so home-relative defaults resolve under the temp
// home alone; set it after Home to pin it.
func Home(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

// FakeHome points $HOME at the fixed path for the test's duration — pinning a
// known, non-existent path for assertions — and clears XDG_CONFIG_HOME like Home.
func FakeHome(t *testing.T, path string) {
	t.Helper()
	t.Setenv("HOME", path)
	t.Setenv("XDG_CONFIG_HOME", "")
}

// NoHome empties $HOME and XDG_CONFIG_HOME for the test's duration, so home
// lookups fail like on a headless machine.
func NoHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
}

// MainHome is Home's TestMain counterpart: it points HOME at one fresh temp dir
// for the whole binary's run — keeping the suite off the machine's real home —
// runs setup, then the tests, and removes the dir afterwards. Returns m.Run's
// exit code for os.Exit.
func MainHome(m *testing.M, setup func()) int {
	home, err := os.MkdirTemp("", "testutil-home")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("XDG_CONFIG_HOME", "")
	if setup != nil {
		setup()
	}

	code := m.Run()
	_ = os.RemoveAll(home)
	return code
}
