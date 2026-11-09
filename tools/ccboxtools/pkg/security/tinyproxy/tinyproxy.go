// Package tinyproxy is the container's proxy process: spawned at boot as a
// dedicated uid, and reloaded when the host rewrites its allow file.
package tinyproxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cmdgen"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/dial"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/inotify"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
)

// The proxy's fixed run: the image-baked binary, conf, and uid, listening on
// pkginfo's shared addr:port.
const (
	bin        = "tinyproxy"
	confPath   = "/etc/ccbox/tinyproxy.conf"
	daemonUser = "tinyproxy"

	listenTimeout = 10 * time.Second
)

// WatchAllowlist reloads the proxy (SIGUSR1) when the host rewrites the allow
// file — the pid func resolves at event time, so respawns reload too.
func WatchAllowlist(ctx context.Context, pid func() int) error {
	return inotify.WatchFile(ctx, pkginfo.ProxyMount, pkginfo.ProxyAllowFile, func() {
		if p := pid(); p != 0 {
			log.WarnErr("reload tinyproxy", syscall.Kill(p, syscall.SIGUSR1))
		}
	})
}

// Cmd builds the proxy's foreground run through the full drop to its dedicated
// uid — an unstarted command for supervise to fork, refusing without the
// host-rendered allow file.
func Cmd(ctx context.Context) (*exec.Cmd, error) {
	if _, err := os.Stat(pkginfo.ProxyAllowPath); err != nil {
		return nil, fmt.Errorf("allow file not bound at %s: %w", pkginfo.ProxyAllowPath, err)
	}
	chain := identity.Drop(daemonUser, bin, "-d", "-c", confPath)
	cmd := cmdgen.Stdio(ctx, chain[0], chain[1:]...)
	cmd.Stdout, cmd.Stderr = logmux.NewMuxer(pkginfo.ProxyMuxName, os.Stdout), logmux.NewMuxer(pkginfo.ProxyMuxName, os.Stderr)
	return cmd, nil
}

// AwaitListen waits the forked proxy's listen out — the kill is its cleanup,
// and the respawn retries.
func AwaitListen(ctx context.Context, cmd *exec.Cmd) error {
	if err := dial.AwaitListen(ctx, pkginfo.ProxyPort, listenTimeout); err != nil {
		log.WarnErr("kill unlistening "+bin, cmd.Process.Kill())
		return err
	}
	return nil
}
