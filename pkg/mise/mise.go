// Package mise manages the devbox's mise config from the host
package mise

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/mfs"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// ConfigName is the mise config's name — the project's at the project root,
	// the user's at userdir.Dir()
	ConfigName = "mise.toml"
	// LockExt is a committed lockfile's ext — mise's own is mise.lock
	LockExt = ".lock"
)

// ProjectConfigPath is dir's mise config path
func ProjectConfigPath(projectDir string) string { return filepath.Join(projectDir, ConfigName) }

// UserConfigPath is the user-level mise config's path, seeded by SeedConfig
func UserConfigPath() string { return filepath.Join(userdir.Dir(), ConfigName) }

// LockPath is the config's committed lockfile path, beside it
func LockPath(configPath string) string {
	return strings.TrimSuffix(configPath, filepath.Ext(configPath)) + LockExt
}

// PickedConfigPath is the config the build picks — the project config over the
// user's; "" when neither is present
func PickedConfigPath(projectDir string) string {
	return osutil.PickLayer(ProjectConfigPath(projectDir), UserConfigPath())
}

// ToolNames reads the picked config's [tools] names, sorted — the mise config is
// the runtime selection's source of truth: the update prompt's pre-fill and the
// image state's input
func ToolNames(projectDir string) ([]string, error) {
	path := PickedConfigPath(projectDir)
	if path == "" {
		return nil, fmt.Errorf("mise: no mise config to read — expected a project config at %s or the user config at %s",
			ProjectConfigPath(projectDir), UserConfigPath())
	}
	body, err := os.ReadFile(path) // #nosec G304 -- the config's own path
	if err != nil {
		return nil, err
	}
	var file struct {
		Tools map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("mise: %s: %w", path, err)
	}
	return slices.Sorted(maps.Keys(file.Tools)), nil
}

// miseImageRe matches the Dockerfile's pure-mise FROM — the lock container must run
// the same mise the devbox image builds from, so the two can't drift.
var miseImageRe = regexp.MustCompile(`(?m)^FROM\s+(\S+)\s+AS\s+mise\s*$`)

// ImageRefFromDockerfile extracts the pinned mise image reference from the Dockerfile.
func ImageRefFromDockerfile(dockerfile []byte) (string, error) {
	match := miseImageRe.FindSubmatch(dockerfile)
	if match == nil {
		return "", errors.New("mise: the Dockerfile has no `FROM <image> AS mise`")
	}
	return string(match[1]), nil
}

// GenerateLock puts the committed lock beside the mise config when the config is
// there, regenerating only when stale — a fresh one is left untouched. Creates both
// the lock and its config-hash
func GenerateLock(ctx context.Context, imageRef, configPath string) error {
	if !osutil.Present(configPath) {
		return nil
	}

	expander := osutil.Expander{
		Source: configPath, Expansion: LockPath(configPath),
		SHAPath: filepath.Join(userdir.Tmp(), "misesha", slug.Path(configPath)+".sha256"),
	}
	isStale, err := expander.IsStale()
	if err != nil {
		return err
	}
	if !isStale {
		return nil
	}

	lockPath := LockPath(configPath)
	log.Infof("generating %s", lockPath)
	ctxD, err := dock.NewCtxD(ctx)
	if err != nil {
		return err
	}
	//nolint:contextcheck // the chain's context.Background cleanups are deliberate: a cancelled ctx can't block cleanup
	lock, err := docker.GenerateMiseLock(ctxD, docker.MiseLockOptions{
		Image:      imageRef,
		ConfigPath: configPath,
	})
	if err != nil {
		return err
	}

	// Expand re-perms to 0644: mise writes its lockfile 0600
	if err := expander.Expand(lock); err != nil {
		return err
	}
	log.Infof("locked %s", lockPath)
	return nil
}

// The mise config lands in the build context at the image's system config: the config
// rides where the Dockerfile COPYs docker/mise/, and the lock sits next to it — mise
// resolves installs through the lockfile beside the config.
const (
	contextConfigPath = "docker/mise/config.toml"
	contextLockPath   = "docker/mise/mise.lock"
)

// independentTools is the image's tools independent of the runtime selection —
// the mise tools picker's second checklist
var independentTools = []Tool{
	{Tool: "bats", Version: "1.13.0"},
	{Tool: "hadolint", Version: "2.14.0"},
	{
		Tool:        "npm:playwright",
		Version:     "latest",
		PostInstall: "playwright install-deps chromium && playwright install chromium",
		Comment:     "# --vnc flag uses this chromium as a browser",
		Hint:        "with chrome",
	},
}

// IndependentTools is the image's independent tools, in render order
func IndependentTools() []Tool { return slices.Clone(independentTools) }

// BuildFS is the build fs for the image's `mise install`: the embed with the
// mise config injected where the Dockerfile COPYs it.
func BuildFS(embed fs.FS, projectDir string) (fs.FS, error) {
	configPath := PickedConfigPath(projectDir)
	if configPath == "" {
		return nil, fmt.Errorf("mise: no mise config to build from — expected a project config at %s or the user config at %s",
			ProjectConfigPath(projectDir), UserConfigPath())
	}
	hostConfigFS, err := configFS(configPath)
	if err != nil {
		return nil, err
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
