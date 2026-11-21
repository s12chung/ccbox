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
// into every cli without skills of its own
func SafeSeedSkills() error { return os.MkdirAll(SkillsDir(), osutil.Dir) }

// SkillsBind binds SkillsDir at the cli's user skills mount when the cli's config
// home lacks skills of its own — an empty dir doesn't count, the cli re-creates
// it each run
func SkillsBind(cliName string) klean.ShareBind {
	if !osutil.DirEmpty(filepath.Join(cli.UserConfigDir(cliName), skillsDirName)) {
		return klean.ShareBind{} // its config home bind already delivers its skills
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
