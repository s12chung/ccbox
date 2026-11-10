// Package vnc serves the image's desktop a run's env asks for
// (pkginfo.VNCConfigEnvVar) — plain, or supervised in the egress wall.
package vnc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cmdgen"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/dial"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/handshake"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/supervise"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc/vncdeps"
)

const (
	// desktopBin is the image's desktop launcher: present = the image serves a GUI over VNC.
	desktopBin = "desktop"
	// sessionMode is desktop.sh's session argv: the session alone, on the bare
	// server's display — the script's default argv is the bare server itself.
	sessionMode = "session"
)

// The desktop's start bounds: the server's listen, and the session's landing —
// the GUI app's download can take minutes, but a hung start must not.
const (
	listenTimeout  = 30 * time.Second
	landingTimeout = 30 * time.Minute
)

// Serve serves a VNC run's desktop, returning the unstarted supervisees keeping
// it up: the bare server first, its listen gating the session landing on it.
func Serve(vncInfo *pkginfo.VNCInfo) ([]*supervise.Respawnable, error) {
	if err := vncdeps.CheckPlaywright(os.Getenv, filepath.Glob); err != nil {
		return nil, err
	}
	server := &supervise.Respawnable{
		Name: "desktop server",
		CmdToStart: func(ctx context.Context) (*exec.Cmd, error) {
			cmd, err := SetupDesktop(ctx, vncInfo)
			if err != nil {
				return nil, err
			}
			return cmd.ExecCmd(ctx), nil
		},
		Ready: func(ctx context.Context, cmd *exec.Cmd) error {
			return dial.AwaitListen(ctx, cmd.Process.Pid, pkginfo.VNCPort, listenTimeout)
		},
	}
	if proxy.Proxied() {
		server.CmdToStart = func(ctx context.Context) (*exec.Cmd, error) { return proxy.IsolatedCmd(ctx, desktopBin) }
	}

	session := &supervise.Respawnable{
		Name:       "desktop session",
		CmdToStart: func(ctx context.Context) (*exec.Cmd, error) { return cmdgen.Stdio(ctx, desktopBin, sessionMode), nil },
	}
	if proxy.Proxied() {
		room := &desktopSession{}
		session.CmdToStart, session.Ready = room.CmdToStart, room.Ready
	}
	return []*supervise.Respawnable{server, session}, nil
}

// desktopSession is the room's desktop session: the fork carries the ready
// pipe, the gate awaits the landing's ready ping.
type desktopSession struct{ ready *handshake.Parent }

func (s *desktopSession) CmdToStart(ctx context.Context) (*exec.Cmd, error) {
	ready, err := handshake.New()
	if err != nil {
		return nil, err
	}
	cmd, err := proxy.NSIsolatedCmd(ctx, "proxy", "desktop")
	if err != nil {
		ready.Close()
		return nil, err
	}
	s.ready, cmd.ExtraFiles = ready, ready.Files()
	return cmd, nil
}

func (s *desktopSession) Ready(ctx context.Context, _ *exec.Cmd) error {
	s.ready.Spawned() // the fork's done — only the child's end remains to write
	defer s.ready.Close()
	// the pipe doubles as a liveness check — the child's death EOFs the await
	return s.ready.AwaitReady(ctx, landingTimeout)
}

// SendReady pings the boot over the fork's ready pipe: the session's install
// is done, the exec is next.
func SendReady() error {
	ready := handshake.ForChild()
	if err := ready.SendReady(); err != nil {
		return err
	}
	ready.Close()
	return nil
}

// DesktopCmd is a VNC run's desktop command: ExecCmd serves the bare server,
// SyscallCmd execs the session — the landing's install already ran.
type DesktopCmd struct{}

// ExecCmd returns the bare server's long-lived command on this process's stdio.
func (DesktopCmd) ExecCmd(ctx context.Context) *exec.Cmd { return cmdgen.Stdio(ctx, desktopBin) }

// SyscallCmd is the landing's exec: the image's session, on the bare server's display.
func (DesktopCmd) SyscallCmd() cmdgen.SyscallCmd {
	return cmdgen.SyscallCmd{Bin: desktopBin, Args: []string{sessionMode}}
}

// SetupDesktop installs a VNC run's GUI app and returns its desktop command —
// the proxied run installs here, inside the egress wall, as the session user.
func SetupDesktop(ctx context.Context, vncInfo *pkginfo.VNCInfo) (DesktopCmd, error) {
	if vncInfo != nil && vncInfo.GUIApp != nil {
		if err := guiapp.Install(ctx, *vncInfo.GUIApp); err != nil {
			return DesktopCmd{}, err
		}
	}
	log.Info("starting vnc (some errors below are harmless)")
	return DesktopCmd{}, nil
}
