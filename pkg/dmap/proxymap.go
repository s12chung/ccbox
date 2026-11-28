package dmap

import (
	"errors"
	"io/fs"
	"os"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/util/diff"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/allowfile"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	cfg *cfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(cfg *cfg.Config) *ProxyMap { return &ProxyMap{cfg: cfg} }

// Options renders the egress wall's docker.ProxyOptions; onChanged reports a
// refresh's change
func (pm *ProxyMap) Options(onChanged func(changed string)) *docker.ProxyOptions {
	return &docker.ProxyOptions{
		HostDir: proxyLiveDir(),
		LogFile: proxyLogPath(pm.cfg.ProjectDir(), pm.cfg.IsVNC()),
		BeforeStart: func() error {
			_, err := pm.Refresh()
			return err
		},
		OnRefresh: func() error {
			changed, err := pm.Refresh()
			if err != nil {
				return err
			}
			if changed != "" {
				onChanged(changed)
			}
			return nil
		},
		// contents go, not the dir: a devbox may still bind it
		OnStop: func() error { return osutil.ClearDir(proxyLiveDir()) },
	}
}

// Refresh re-renders the allow file from the config, rewrites it, and shows diff —
// the container's watcher reloads the proxy off the rename.
func (pm *ProxyMap) Refresh() (string, error) {
	fresh := allowfile.Render(pm.cfg.AllowlistExpanded())
	changed, err := pm.allowlistDiff(fresh)
	if err != nil {
		return "", err
	}
	if err := osutil.AtomicWriteFile(proxyAllowPath(), fresh); err != nil {
		return "", err
	}
	return changed, nil
}

// allowlistDiff diffs in bare domains. A missing file diffs from nothing.
func (pm *ProxyMap) allowlistDiff(fresh []byte) (string, error) {
	body, err := os.ReadFile(proxyAllowPath()) // #nosec G304 -- the proxy's own live file
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	liveDomains, err := allowfile.Parse(string(body))
	if err != nil {
		return "", err
	}
	freshDomains, err := allowfile.Parse(string(fresh))
	if err != nil {
		return "", err // unreachable: fresh is the map's own render
	}
	return diff.Sets(liveDomains, freshDomains), nil
}
