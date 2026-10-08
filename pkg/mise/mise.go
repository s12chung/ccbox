// Package mise manages the devbox's mise config from the host
package mise

import (
	"context"
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/mfs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// ConfigName is the project mise config's name at the project root
	ConfigName = ".ccbox-mise.toml"
	// UserConfigName is the user-level mise config's name at userdir.Dir()
	UserConfigName = "ccbox-mise.toml"
	// LockExt is a committed lockfile's ext — mise's own is mise.lock
	LockExt = ".lock"
)

// ProjectConfigPath is dir's mise config path
func ProjectConfigPath(projectDir string) string { return filepath.Join(projectDir, ConfigName) }

// UserConfigPath is the user-level mise config's path, seeded by SeedConfig
func UserConfigPath() string { return filepath.Join(userdir.Dir(), UserConfigName) }

// LockPath is the config's committed lockfile path, beside it
func LockPath(configPath string) string {
	return strings.TrimSuffix(configPath, filepath.Ext(configPath)) + LockExt
}

// miseImageRe matches the Dockerfile's pure-mise FROM — the lock container must run
// the same mise the devbox image builds from, so the two can't drift.
var miseImageRe = regexp.MustCompile(`(?m)^FROM\s+(\S+)\s+AS\s+mise\s*$`)

// ImageFromDockerfile extracts the pinned mise image reference from the Dockerfile.
func ImageFromDockerfile(dockerfile []byte) (string, error) {
	match := miseImageRe.FindSubmatch(dockerfile)
	if match == nil {
		return "", errors.New("mise: the Dockerfile has no `FROM <image> AS mise`")
	}
	return string(match[1]), nil
}

// GenerateLock puts the committed lock beside the mise config when the config is
// there, regenerating only when stale — a fresh one is left untouched. Creates both
// the lock and its config-hash
func GenerateLock(ctx context.Context, image, configPath string) error {
	if !ioutil.Present(configPath) {
		return nil
	}

	expander := ioutil.Expander{Source: configPath, Expansion: LockPath(configPath)}
	isStale, err := expander.IsStale()
	if err != nil {
		return err
	}
	if !isStale {
		return nil
	}

	outDir, err := os.MkdirTemp("", "ccbox-mise-lock-")
	if err != nil {
		return err
	}
	defer log.Defer("clean mise lock dir", func() error { return os.RemoveAll(outDir) })
	ctxD, err := dock.NewCtxD(ctx)
	if err != nil {
		return err
	}
	//nolint:contextcheck // the chain's context.Background cleanups are deliberate: a cancelled ctx can't block cleanup
	lock, err := docker.GenerateMiseLock(ctxD, docker.MiseLockOptions{
		Image:      image,
		ConfigPath: configPath,
		OutDir:     outDir,
		UID:        os.Getuid(),
	})
	if err != nil {
		return err
	}

	// Expand re-perms to 0644: mise writes its lockfile 0600
	if err := expander.Expand(lock); err != nil {
		return err
	}
	log.Infof("locked %s", LockPath(configPath))
	return nil
}

// The mise config lands in the build context at the image's system config: the config
// rides where the Dockerfile COPYs docker/mise/, and the lock sits next to it — mise
// resolves installs through the lockfile beside the config.
const (
	contextConfigPath = "docker/mise/config.toml"
	contextLockPath   = "docker/mise/mise.lock"
)

// defaultConfig is the seeded default config
//
//go:embed config.toml
var defaultConfig []byte

// SeedConfig safe-seeds the user-level mise config with the pinned defaults
func SeedConfig(path string) error { return fsync.SafeFile(path, defaultConfig) }

// BuildFS is the build fs for the image's `mise install`: the embed with the
// mise config injected where the Dockerfile COPYs it
func BuildFS(embed fs.FS, projectDir string) (fs.FS, error) {
	var hostConfigFS fs.FS
	for _, configPath := range []string{UserConfigPath(), ProjectConfigPath(projectDir)} {
		if !ioutil.Present(configPath) {
			continue
		}
		var err error
		hostConfigFS, err = configFS(configPath)
		if err != nil {
			return nil, err
		}
	}
	if hostConfigFS == nil {
		return nil, errors.New("no mise config found")
	}

	merged, err := mfs.NewFS(embed)
	if err != nil {
		return nil, err
	}
	if err := merged.Merge(hostConfigFS); err != nil {
		return nil, err
	}
	return merged, nil
}

// configFS reads the config and its committed lock into the build context's files
func configFS(configPath string) (mfs.MapFS, error) {
	config, err := os.ReadFile(configPath) // #nosec G304 -- the config's own path
	if err != nil {
		return nil, err
	}
	lock, err := os.ReadFile(LockPath(configPath)) // #nosec G304 -- the config's own lock path
	if err != nil {
		return nil, err
	}
	return mfs.MapFS{
		contextConfigPath: {Data: config},
		contextLockPath:   {Data: lock},
	}, nil
}
