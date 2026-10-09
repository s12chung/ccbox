package prompt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
)

var confirmFileName = "confirm.json"

// confirmPath is confirm.json's path
func confirmPath() string { return filepath.Join(userdir.ConfigDir(), confirmFileName) }

// confirm is confirm.json's shape
type confirm struct {
	ProjectDirs []string `json:"project_dirs"`
}

// loadConfirm reads confirm.json; a missing file is nothing confirmed yet
func loadConfirm() (confirm, error) {
	body, err := os.ReadFile(confirmPath()) // #nosec G304 -- the package's own file
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return confirm{}, nil
	case err != nil:
		return confirm{}, err
	}
	var c confirm
	if err := json.Unmarshal(body, &c); err != nil {
		return confirm{}, fmt.Errorf("%s: %w", confirmPath(), err)
	}
	return c, nil
}

// save appends dir to the recorded dirs and writes confirm.json
func (c *confirm) saveProjectDir(dir string) error {
	c.ProjectDirs = append(c.ProjectDirs, dir)
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return osutil.AtomicWriteFile(confirmPath(), append(body, '\n'))
}
