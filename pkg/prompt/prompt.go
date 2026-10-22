// Package prompt asks the questions ccbox won't proceed without an answer to.
// An accepted answer is recorded in ConfigDir's confirm.json — the stuff
// confirmed — so its question never asks again.
package prompt

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/size"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/pkg/util/term"
)

// ProjectDirLimit is the project dir size ProjectDirSize gates over: 1 GiB
const ProjectDirLimit = 1 << 30

var (
	runFn   = pick.Runner.Run // indirected so tests can stub out the prompt run.
	dirOver = size.DirOver    // indirected so tests can stub out the sizing.
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
func HarnessCLI(clis []cli.CLI) (string, error) {
	var picked cliModel
	err := runFn(pick.NewRunner(pick.RunnerOptions{
		Header:          "Select a harness CLI",
		Subtext:         []string{fmt.Sprintf("see %s to plug your own", userdir.Tilde(filepath.Join(clitmpl.UserDir(), "README.md")))},
		SubmitLabel:     "Save to " + userdir.Tilde(cfg.UserConfigFile()),
		RequireSelected: true,
	}, pick.NewPicker(cliModels(clis))), &picked)
	if err != nil {
		return "", err
	}
	if picked.Name() == "" { // no terminal to show the picker
		return "", fmt.Errorf("no harness CLI selected: rerun in a terminal to pick one, or set cli in %s", cfg.UserConfigFile())
	}
	return picked.Name(), nil
}

// cliModel is the cli.CLI as a pick.Model
type cliModel struct{ cli.CLI }

func (m cliModel) Name() string { return m.CLI.Name }
func (m cliModel) Hint() string { return "" }

func cliModels(clis []cli.CLI) []cliModel {
	return slicex.Map(clis, func(c cli.CLI) cliModel { return cliModel{c} })
}
