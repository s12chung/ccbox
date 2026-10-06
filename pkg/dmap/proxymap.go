package dmap

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/proxy"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	cfg *projectcfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(cfg *projectcfg.Config) *ProxyMap { return &ProxyMap{cfg: cfg} }

// Options renders the egress wall's docker.ProxyOptions
func (pm *ProxyMap) Options() docker.ProxyOptions {
	allowFileContents := proxy.Render(pm.cfg.AllowlistExpanded())
	return docker.ProxyOptions{
		ConfigFS:      tinyproxy.Config,
		ConfigFileMap: map[string][]byte{pkginfo.ProxyAllowFile: allowFileContents},
		OnStart: func() error {
			return ioutil.SafeWriteFile(proxyAllowHostPath(), allowFileContents)
		},
		OnStop: func() error {
			err := os.Remove(proxyAllowHostPath())
			if errors.Is(err, fs.ErrNotExist) {
				return nil // a missing file is already clean
			}
			return err
		},
	}
}

// proxyAllowHostDir is the proxy's live dir on the host: ~/.ccbox/tmp/proxy. proxyStart
// writes the rendered allow bytes here at proxy start, and every devbox binds the dir
// read-only for the allowlist command — a dir bind tracks the changing file as the
// running proxy's current allowlist.
func proxyAllowHostDir() string { return filepath.Join(userdir.Tmp(), "proxy") }

// proxyAllowHostPath is the rendered allow file within proxyAllowHostDir
func proxyAllowHostPath() string { return filepath.Join(proxyAllowHostDir(), pkginfo.ProxyAllowFile) }
