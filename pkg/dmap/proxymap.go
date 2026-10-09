package dmap

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/diff"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	cfg *projectcfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(cfg *projectcfg.Config) *ProxyMap { return &ProxyMap{cfg: cfg} }

// Options renders the egress wall's docker.ProxyOptions; foreground is the optional
// stream — nil runs ride the session's log file only
func (pm *ProxyMap) Options(foreground *tinyproxy.Log) *docker.ProxyOptions {
	proxyConfigMap := map[string][]byte{
		tinyproxy.ConfFile:     tinyproxy.MustConf(),
		pkginfo.ProxyAllowFile: proxy.Render(pm.cfg.AllowlistExpanded()),
	}
	writeConfigMap := func() error {
		for name, body := range proxyConfigMap {
			// atomically for reload
			if err := osutil.AtomicWriteFile(filepath.Join(proxyLiveDir(), name), body); err != nil {
				return err
			}
		}
		return nil
	}
	return &docker.ProxyOptions{
		HostDir:     proxyLiveDir(),
		HoldersDir:  proxyHoldersDir(),
		LogFile:     proxyLogPath(),
		Log:         foreground,
		BeforeStart: writeConfigMap,
		OnRefresh: func() error {
			changed, err := pm.allowlistDiff(proxyConfigMap[pkginfo.ProxyAllowFile])
			if err != nil {
				return err
			}
			if err := writeConfigMap(); err != nil {
				return err
			}
			if changed != "" {
				log.Info("allow list changed:\n" + changed)
			}
			return nil
		},
		// contents go, not the dir: a devbox may still bind it
		OnStop: func() error { return osutil.ClearDir(proxyLiveDir()) },
	}
}

// allowlistDiff diffs in bare domains — like the config, never the file's regex. A missing
// file diffs from nothing.
func (pm *ProxyMap) allowlistDiff(fresh []byte) (string, error) {
	body, err := os.ReadFile(proxyAllowPath()) // #nosec G304 -- the proxy's own live file
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	liveDomains, err := proxy.Parse(string(body))
	if err != nil {
		return "", err
	}
	freshDomains, err := proxy.Parse(string(fresh))
	if err != nil {
		return "", err // unreachable: fresh is the map's own render
	}
	return diff.Lines(liveDomains, freshDomains), nil
}

// proxyLiveDir is the proxy's live dir on the host: ~/.ccbox/tmp/proxy. A dir bind tracks
// the changing files as the running proxy's current configs.
func proxyLiveDir() string { return filepath.Join(userdir.Tmp(), "proxy") }

// proxyHoldersDir is the proxy session's holder flock dir on the host: ~/.ccbox/tmp/proxy-holders
func proxyHoldersDir() string { return filepath.Join(userdir.Tmp(), "proxy-holders") }

// proxyAllowPath is the rendered allow file within proxyLiveDir
func proxyAllowPath() string { return filepath.Join(proxyLiveDir(), pkginfo.ProxyAllowFile) }
