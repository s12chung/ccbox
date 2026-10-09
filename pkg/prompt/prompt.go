// Package prompt asks the questions ccbox won't proceed without an answer to.
// An accepted answer is recorded in ConfigDir's confirm.json — the stuff
// confirmed — so its question never asks again.
package prompt

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/models/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/size"
	"github.com/s12chung/ccbox/pkg/util/term"
)

// ProjectDirLimit is the project dir size ProjectDirSize gates over: 1 GiB
const ProjectDirLimit = 1 << 30

var (
	selectFn = pick.Select  // indirected so tests can stub out the picker.
	dirOver  = size.DirOver // indirected so tests can stub out the sizing.
)

// ProjectDirSize gates projectDir when its size is over ProjectDirLimit. A recorded
// dir — a past yes — skips the walk entirely.
func ProjectDirSize(projectDir string) error {
	confirmed, err := loadConfirm()
	if err != nil {
		return err
	}
	if slices.Contains(confirmed.ProjectDirs, projectDir) {
		return nil
	}
	over, err := dirOver(projectDir, ProjectDirLimit)
	if err != nil {
		return err
	}
	if !over {
		return nil
	}
	question := fmt.Sprintf("%s is over %d GB--proceed?", userdir.Tilde(projectDir), ProjectDirLimit>>30)
	if term.Confirm(question) {
		return confirmed.saveProjectDir(projectDir)
	}
	return fmt.Errorf("%s is over %d GB and wasn't confirmed: rerun to confirm it, or add it to %s",
		userdir.Tilde(projectDir), ProjectDirLimit>>30, userdir.Tilde(confirmPath()))
}

// HarnessCLI asks which harness CLI to seed the user config with. No terminal to
// show the picker errors out — a CLI has no default to keep.
func HarnessCLI(cliNames []string) (string, error) {
	name, err := selectFn(
		"Select a harness CLI",
		[]string{
			fmt.Sprintf("(stored in %s)", userdir.Tilde(projectcfg.UserConfigFile())),
			fmt.Sprintf("see %s to plug your own", userdir.Tilde(filepath.Join(clitmpl.UserDir(), "README.md"))),
		},
		cliNames)
	if err != nil {
		return "", err
	}
	if name == "" { // no terminal to show the picker
		return "", fmt.Errorf("no harness CLI selected: rerun in a terminal to pick one, or set cli in %s", projectcfg.UserConfigFile())
	}
	return name, nil
}
