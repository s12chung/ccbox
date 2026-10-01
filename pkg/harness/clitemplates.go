package harness

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/s12chung/firm"
	"gopkg.in/yaml.v3"
)

// Tree is a CLI templates tree — clis/<name>/CLI.yaml
type Tree struct {
	fsys        fs.FS
	fromUserDir bool
}

// embedFS is the compile-time embedded CLI templates tree: this package's clis/.
//
//go:embed clis
var embedFS embed.FS

// userFS DOES NOT detect whether userConfigDir exists; this should be detected on init()--see package NOTE
func userFS() fs.FS { return os.DirFS(userConfigDir) }

// embedTree is the compile-time embedded CLI templates tree.
func embedTree() Tree { return Tree{fsys: embedFS} }

// userTree is the host's user-defined CLI templates tree at userConfigDir.
func userTree() Tree { return Tree{fsys: userFS(), fromUserDir: true} }

// load parses each clis/<cli> dir's CLI.yaml into a CLI named <cli>, ordered by name,
// ignoring files dropped into clis/.
func (t Tree) load() ([]CLI, []error, error) {
	glob := path.Join(clisDir, "*")
	paths, err := fs.Glob(t.fsys, glob)
	if err != nil {
		// unreachable: glob never changes
		return nil, nil, err
	}

	clis := make([]CLI, 0, len(paths))
	var warns []error
	for _, p := range paths {
		info, err := fs.Stat(t.fsys, p)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			continue
		}
		c, err := t.loadCLI(p)
		if err != nil {
			err = fmt.Errorf("%s: %w", path.Base(p), err)
			if !t.fromUserDir {
				return nil, nil, err
			}
			warns = append(warns, err)
			continue
		}
		clis = append(clis, c)
	}

	// an embed tree is a compile-time constant: empty is a build bug; a user tree may be empty
	if len(clis) == 0 && !t.fromUserDir {
		return nil, nil, fmt.Errorf("harness: no %s found", glob)
	}
	return clis, warns, nil
}

// loadCLI reads, parses, and validates the CLI.yaml in the cli dir p.
func (t Tree) loadCLI(p string) (CLI, error) {
	yamlPath := path.Join(p, cliYAML)
	body, err := fs.ReadFile(t.fsys, yamlPath)
	if errors.Is(err, fs.ErrNotExist) {
		return CLI{}, fmt.Errorf("%s: not found", yamlPath) // a stray dir: no CLI.yaml
	}
	if err != nil {
		return CLI{}, err
	}
	c, err := parse(yamlPath, body)
	if err != nil {
		return CLI{}, err
	}
	if err := t.validateConfigDir(p); err != nil {
		return CLI{}, err
	}
	c.fromUserDir = t.fromUserDir
	return c, nil
}

// parse decodes one CLI.yaml into its CLI, named after its directory.
func parse(p string, body []byte) (CLI, error) {
	var c CLI
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return CLI{}, fmt.Errorf("harness: parse %s: %w", p, err)
	}

	c = defaulted(p, c)
	if errMap := firm.ValidateAny(c); errMap != nil { // fail at startup, not on first use
		return CLI{}, fmt.Errorf("harness: parse %s: %w", p, errMap)
	}
	return c, nil
}

func cliNameFromPath(p string) string { return path.Base(path.Dir(p)) }

func defaulted(p string, c CLI) CLI {
	c.Name = cliNameFromPath(p)
	return c
}

// validateConfigDir checks <cli dir>/config: for user trees it may be absent, but must not be a file;
// embed trees also require existence — their contents are compile-time constants.
func (t Tree) validateConfigDir(dir string) error {
	info, err := fs.Stat(t.fsys, path.Join(dir, seedTemplateConfigDir))
	switch {
	case t.fromUserDir && errors.Is(err, fs.ErrNotExist):
		return nil // seeded fresh by UserCLIConfigSeedFS
	case err != nil:
		return err
	case !info.IsDir():
		return fmt.Errorf("%s: not a directory", path.Join(dir, seedTemplateConfigDir))
	}
	return nil
}
