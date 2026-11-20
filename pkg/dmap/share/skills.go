package share

import (
	"os"
	"path"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// SafeSeedSkills seeds the shared skills dir when missing: an empty pool binding
// into every cli without its own skills dir
func SafeSeedSkills() error { return os.MkdirAll(SkillsDir(), osutil.Dir) }

// SkillsBind binds SkillsDir at the cli's user skills mount when the cli's config
// home lacks its own; the zero bind shares nothing — its config home bind already
// delivers its skills
func SkillsBind(cliName string) klean.ShareBind {
	if osutil.Present(filepath.Join(cli.UserConfigDir(cliName), skillsDirName)) {
		return klean.ShareBind{}
	}
	return klean.ShareBind{
		HostPath:      SkillsDir(),
		ContainerPath: path.Join(pkginfo.ContainerHome, cli.MustFor(cliName).ConfigHomeMount, skillsDirName),
	}
}

// SkillsDir is the shared skills dir: ~/.ccbox/skills
func SkillsDir() string { return filepath.Join(userdir.Dir(), skillsDirName) }

// skillsDirName names the skills dir within the shared root and each cli's config home
const skillsDirName = "skills"
