package pkger

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestFor(t *testing.T) {
	npm := For(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}})
	assert.IsType(t, Npm{}, npm)
	assert.Equal(t, "claude", npm.Name())

	vu := For(pkginfo.PkgInfo{
		Name:       "grok",
		VersionURL: &pkginfo.VersionURL{URL: "https://x", LinuxX64URL: "https://x", LinuxArm64URL: "https://x"},
	})
	assert.IsType(t, VersionURL{}, vu)
	assert.Equal(t, "grok", vu.Name())
}

func TestPkgDir(t *testing.T) {
	d := PkgDir{Pkger: For(pkginfo.PkgInfo{Name: "claude", Npm: &pkginfo.Npm{Package: "a"}}), Root: "/opt/ccbox/clis"}

	assert.Equal(t, "/opt/ccbox/clis/claude", d.Dir())
	assert.Equal(t, "/opt/ccbox/clis/claude/1.2.3", d.VersionDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/.tmp-1.2.3", d.TmpDir("1.2.3"))
	assert.Equal(t, "/opt/ccbox/clis/claude/current", d.Current())
	assert.Equal(t, "/opt/ccbox/clis/bin/claude", d.BinLink())
	assert.Equal(t, "../claude/current/bin/claude", d.BinLinkTarget())
}
