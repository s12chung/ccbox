// Package clitmpl loads the trees ccbox's harnesses are defined by and seeded from.
//
// clis naming, on two axes:
//   - source: "embed" (compile-time) or "user" (host, under userdir.Dir())
//   - target: "CLI templates" — the clis tree: <root>/clis/<name>/CLI.yaml plus <root>/clis/<name>/config/
//     "CLI config" — the CLI's live config: ~/.ccbox/<cli_name>
//
// Seeding seeds the CLI config with the template.
package clitmpl

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/mfs"
	"github.com/s12chung/ccbox/pkg/util/must"
)

const (
	clisDir               = "clis"
	cliYAML               = "CLI.yaml"
	seedTemplateConfigDir = "config"
)

// userConfigDir is the user config dir: ~/.ccbox/config.
var userConfigDir = userdir.ConfigDir()

// SetUserConfigDir points the user config dir at dir; tests redirect the user
// clis tree at a temp tree with it.
func SetUserConfigDir(dir string) { userConfigDir = dir }

// Tree is a CLI templates tree — clis/<name>/CLI.yaml
type Tree struct {
	fsys          fs.FS
	isUserDefined bool
}

// embedFS is the compile-time embedded CLI templates tree: this package's clis/.
//
//go:embed clis
var embedFS embed.FS

// userFS DOES NOT detect whether userConfigDir exists; this should be detected on init()--see the cli package NOTE
func userFS() fs.FS { return os.DirFS(userConfigDir) }

// EmbedTree is the compile-time embedded CLI templates tree.
func EmbedTree() Tree { return Tree{fsys: embedFS} }

// UserTree is the host's user-defined CLI templates tree at userConfigDir.
func UserTree() Tree { return Tree{fsys: userFS(), isUserDefined: true} }

// UserDir is the host dir of user-defined CLI templates: ~/.ccbox/config/clis.
func UserDir() string { return filepath.Join(userConfigDir, clisDir) }

// Load parses each clis/<cli> dir's CLI.yaml into a T via parse — which owns the
// struct and its validation — named after its dir, ordered by name, ignoring files
// dropped into clis/. Embed-tree errors are fatal; user-tree errors are warnings.
func Load[T any](t Tree, parse func(name string, body []byte, isUserDefined bool) (T, error)) ([]T, []error, error) {
	glob := path.Join(clisDir, "*")
	paths, err := fs.Glob(t.fsys, glob)
	if err != nil {
		// unreachable: glob never changes
		return nil, nil, err
	}

	clis := make([]T, 0, len(paths))
	var warns []error
	for _, p := range paths {
		info, err := fs.Stat(t.fsys, p)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			continue
		}

		name, body, cliErr := t.loadCLI(p)
		var c T
		if cliErr == nil {
			c, cliErr = parse(name, body, t.isUserDefined)
		}
		if cliErr != nil {
			cliErr = fmt.Errorf("%s: %w", name, cliErr)
			if !t.isUserDefined {
				return nil, nil, cliErr
			}
			warns = append(warns, cliErr)
			continue
		}
		clis = append(clis, c)
	}

	// an embed tree is a compile-time constant: empty is a build bug; a user tree may be empty
	if len(clis) == 0 && !t.isUserDefined {
		return nil, nil, fmt.Errorf("clitmpl: no %s found", glob)
	}
	return clis, warns, nil
}

// loadCLI loads one cli dir p's parse inputs, named after its dir: its config/ is
// validated beforehand, then its CLI.yaml is read.
func (t Tree) loadCLI(p string) (string, []byte, error) {
	name := path.Base(p)
	if err := t.validateConfigDir(p); err != nil {
		return name, nil, err
	}
	yamlPath := path.Join(p, cliYAML)
	body, err := fs.ReadFile(t.fsys, yamlPath)
	if errors.Is(err, fs.ErrNotExist) {
		err = fmt.Errorf("%s: not found", yamlPath) // a stray dir: no CLI.yaml
	}
	return name, body, err
}

// validateConfigDir checks <cli dir>/config: for user trees it may be absent, but must not be a file;
// embed trees also require existence — their contents are compile-time constants.
func (t Tree) validateConfigDir(dir string) error {
	info, err := fs.Stat(t.fsys, path.Join(dir, seedTemplateConfigDir))
	switch {
	case t.isUserDefined && errors.Is(err, fs.ErrNotExist):
		return nil // seeded fresh by UserCLIConfigSeedFS
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("%s: not a directory", path.Join(dir, seedTemplateConfigDir))
	}
	return nil
}

// UserCLIConfigSeedFS returns the seed fs for the CLI config: the cli's template
// config tree — embed or user tree per isUserDefined.
func UserCLIConfigSeedFS(cliName string, isUserDefined bool) fs.FS {
	fsTemplateConfigPath := path.Join(clisDir, cliName, seedTemplateConfigDir)
	fsys := fs.FS(embedFS)
	if isUserDefined {
		// mergedFS.MkdirAll ensures fsTemplateConfigPath exists for the caller, so the fs.Sub()
		// below doesn't panic: Load skips user clis whose clis/<cliName>/config is a file
		mergedFS := mfs.MustNewFS(userFS())
		must.Do(mergedFS.MkdirAll(fsTemplateConfigPath))
		fsys = mergedFS
	}
	return must.Get(fs.Sub(fsys, fsTemplateConfigPath))
}

//go:embed user-clis
var userSeed embed.FS

// UserSeedFS returns the embedded tree seeded onto a fresh UserDir().
func UserSeedFS() fs.FS { return must.Get(fs.Sub(userSeed, "user-clis")) }
