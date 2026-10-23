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

// SidecarsPath is the config's dependency sidecars' dir — mise's sidecar root for a
// lockfile beside the config.
func SidecarsPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), ".mise", "locks")
}

// PickedConfigPath is the config the build picks — the project config over the
// user's; "" when neither is present
func PickedConfigPath(projectDir string) string {
	return osutil.PickLayer(ProjectConfigPath(projectDir), UserConfigPath())
}

// ToolState is the picked config's [tools] state
type ToolState struct {
	Pins         map[string]string
	PostInstalls map[string]string
	// VersionFunc fetches a name's latest version
	VersionFunc func(name string) (string, error)
}

// PinFor is the name's pin — written pins as-is, the rest fetched; an
// unresolved pin errors, never renders empty
func (s ToolState) PinFor(name string) (string, error) {
	if pin := s.Pins[name]; pin != "" {
		return pin, nil
	}
	if s.VersionFunc != nil {
		pin, err := s.VersionFunc(name)
		if err != nil {
			return "", err
		}
		if pin != "" {
			return pin, nil
		}
	}
	return "", fmt.Errorf("mise: %s: no pin resolved", name)
}

// SeedToolState is a fresh seed's ToolState: the dev tools as the checks'
// prefill, unpinned — PinFor fetches their versions fresh
func SeedToolState() ToolState {
	pins := make(map[string]string, len(devTools))
	for _, tool := range devTools {
		pins[tool.Name] = ""
	}
	return ToolState{Pins: pins}
}

// ReadToolState reads the picked config's [tools] state — the runtime
// selection's source of truth
func ReadToolState(projectDir string) (ToolState, error) {
	path := PickedConfigPath(projectDir)
	if path == "" {
		return ToolState{}, fmt.Errorf("mise: no mise config to read — expected a project config at %s or the user config at %s",
			ProjectConfigPath(projectDir), UserConfigPath())
	}
	body, err := os.ReadFile(path) // #nosec G304 -- the config's own path
	if err != nil {
		return ToolState{}, err
	}
	var file struct {
		Tools map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(body, &file); err != nil {
		return ToolState{}, fmt.Errorf("mise: %s: %w", path, err)
	}
	state := ToolState{
		Pins:         make(map[string]string, len(file.Tools)),
		PostInstalls: make(map[string]string),
	}
	for name, entry := range file.Tools {
		switch entry := entry.(type) {
		case string:
			state.Pins[name] = entry
		case map[string]any: // the postinstall inline table
			state.Pins[name], _ = entry["version"].(string)
			if post, ok := entry["postinstall"].(string); ok {
				state.PostInstalls[name] = post
			}
		}
	}
	return state, nil
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
// there, regenerating only when stale — a fresh one is left untouched. Creates the
// lock, its sidecars and the config-hash
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
	if err := expander.Expand(lock.Lock); err != nil {
		return err
	}
	return replaceSidecars(configPath, lock.Sidecars)
}

// replaceSidecars rewrites the config's sidecars with the generation's. mise prunes
// moved sidecars only at its own save, so clearing first drops any version dir the
// regeneration left behind.
func replaceSidecars(configPath string, sidecars map[string][]byte) error {
	sidecarsPath := SidecarsPath(configPath)
	if err := os.RemoveAll(sidecarsPath); err != nil {
		return err
	}
	for _, rel := range slices.Sorted(maps.Keys(sidecars)) {
		if err := osutil.SafeWriteFile(filepath.Join(sidecarsPath, rel), sidecars[rel]); err != nil {
			return err
		}
	}
	return nil
}

// The mise config lands in the build context at the image's system config: the config
// where the Dockerfile COPYs docker/mise/, the lock beside it, and the sidecars under
// .mise/locks — mise resolves installs through the lockfile beside the config and its
// sidecar root.
const (
	contextConfigPath   = "docker/mise/config.toml"
	contextLockPath     = "docker/mise/mise.lock"
	contextSidecarsPath = "docker/mise/.mise/locks"
)

// devTools is the dev-workflow CLIs every devbox gets — rendered after the
// runtimes, ahead of the independent tools
var devTools = []Tool{
	{Name: "ripgrep"},
	{Name: "shellcheck"},
	{Name: "jq"},
	{Name: "yq"},
}

// DevTools is the dev-workflow CLIs in every config, in render order
func DevTools() []Tool { return slices.Clone(devTools) }

// independentTools is the image's tools independent of the runtime selection —
// the mise tools picker's third checklist, the rendered region's tail
var independentTools = []Tool{
	{Name: "bats"},
	{Name: "hadolint"},
	{
		Name:               "npm:playwright",
		PostInstallCmdTmpl: "playwright install-deps chromium && playwright install chromium",
		Comment:            "# --vnc flag uses this chromium as a browser",
		Hint:               "with chrome",
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

// configFS reads the config, its committed lock and its dependency sidecars into the
// build context's files
func configFS(configPath string) (mfs.MapFS, error) {
	config, err := os.ReadFile(configPath) // #nosec G304 -- the config's own path
	if err != nil {
		return nil, err
	}
	lock, err := os.ReadFile(LockPath(configPath)) // #nosec G304 -- the config's own lock path
	if err != nil {
		return nil, err
	}
	context := mfs.MapFS{
		contextConfigPath: {Data: config},
		contextLockPath:   {Data: lock},
	}
	sidecars, err := sidecarsMap(configPath)
	if err != nil {
		return nil, err
	}
	for rel, body := range sidecars {
		context[contextSidecarsPath+"/"+rel] = &mfs.MapFile{Data: body}
	}
	return context, nil
}

// sidecarsMap reads the config's committed sidecar files, keyed by their paths under
// the sidecar root — the generation's own keying. A config without sidecar-backed
// tools reads none.
func sidecarsMap(configPath string) (map[string][]byte, error) {
	files := map[string][]byte{}
	sidecarsPath := SidecarsPath(configPath)
	if osutil.Missing(sidecarsPath) {
		return files, nil
	}
	sidecarsFS := os.DirFS(sidecarsPath)
	err := fs.WalkDir(sidecarsFS, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(sidecarsFS, rel)
		if err != nil {
			return err
		}
		files[rel] = body
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
