// Package seed lays embedded config trees onto host directories, backing up
// any files it would overwrite.
package seed

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/perm"
)

// SeedClaudeConfig seeds the Claude config tree, renaming root CLAUDE.user.md -> CLAUDE.md.
func SeedClaudeConfig(src fs.FS, destDir string) ([]string, error) {
	return seedTree(src, destDir, map[string]string{"CLAUDE.user.md": "CLAUDE.md"})
}

// SeedCodexConfig seeds the Codex config tree, renaming root AGENTS.user.md -> AGENTS.md.
func SeedCodexConfig(src fs.FS, destDir string) ([]string, error) {
	return seedTree(src, destDir, map[string]string{"AGENTS.user.md": "AGENTS.md"})
}

// SeedProject seeds a project tree as-is (no filename remapping).
func SeedProject(src fs.FS, destDir string) ([]string, error) {
	return seedTree(src, destDir, nil)
}

// seedTree copies every file in src into destDir (creating it), preserving the tree
// and applying renames (source path -> destination name) where present. A destination
// already matching the source is left untouched. Other existing
// destination files are backed up to <base>.old<ext> before being overwritten; the
// backed-up paths are returned. A pre-existing backup is never clobbered — it's a hard error.
func seedTree(src fs.FS, destDir string, renames map[string]string) (renamed []string, err error) {
	err = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}

		name := p
		if to, ok := renames[p]; ok {
			name = to
		}
		dest := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(dest), perm.Dir); err != nil {
			return err
		}
		if existing, err := os.ReadFile(dest); err == nil {
			if bytes.Equal(existing, body) {
				return nil
			}
			backup := backupPath(dest)
			if _, err := os.Stat(backup); err == nil {
				return fmt.Errorf("seed: backup already exists, refusing to overwrite: %s", backup)
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(dest, backup); err != nil {
				return err
			}
			renamed = append(renamed, backup)
		}
		return os.WriteFile(dest, body, fileMode(dest))
	})
	return renamed, err
}

// backupPath inserts ".old" before the extension: foo/bar.json -> foo/bar.old.json.
func backupPath(p string) string {
	ext := filepath.Ext(p)
	return strings.TrimSuffix(p, ext) + ".old" + ext
}

// fileMode makes shell scripts executable; everything else is a regular file.
func fileMode(p string) os.FileMode {
	if filepath.Ext(p) == ".sh" {
		return perm.ExecFile
	}
	return perm.File
}
