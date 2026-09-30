// Package guiapp wires the GUI app the run's env names (pkginfo.GUIAppEnvVar):
// the boot installs it into the apps volume and writes its session menu entry,
// while the exec entry points launch the installed app.
package guiapp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/fsutil"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
)

// logPath is the autostart's scratch log: the launch record plus the app's own
// streams, which the X session would otherwise drop.
const logPath = "/tmp/guiapp.log"

// pollInterval waits out the boot's install, matching the download's pace.
const pollInterval = time.Second

// desktopEntryFmt is the session menu entry: Name carries the app's own
// branding, Exec launches through the container's own launcher.
const desktopEntryFmt = `[Desktop Entry]
Name=%s
Exec=/usr/local/bin/ccboxtools guiapp exec %%U
Terminal=false
Type=Application
Categories=Development;
`

const fileMode os.FileMode = 0o644

// Install installs the GUI app the run's env names and writes its session menu
// entry — best-effort like the desktop: the session works without the app, so
// failures warn instead of failing the boot. The session's XDG autostart waits
// out the download rather than racing it.
func Install() {
	body := os.Getenv(pkginfo.GUIAppEnvVar)
	if body == "" {
		log.Warnf("%s is not set; skipping the GUI app", pkginfo.GUIAppEnvVar)
		return
	}
	info, err := pkginfo.GUIFromJSON(body)
	if err != nil {
		log.WarnErr("install gui app", err)
		return
	}
	if err := writeDesktopEntry(info); err != nil {
		log.WarnErr("write gui app menu entry", err)
	}
	if err := install.RunPkgInfo(info.PkgInfo, install.AppsRoot); err != nil {
		log.WarnErr("install gui app", err)
	}
}

// writeDesktopEntry writes the app's menu entry under the home's applications
// dir, overwriting any stale copy — ~/.local is a per-project volume at runtime,
// so a start-time write lands where the session reads it.
func writeDesktopEntry(info pkginfo.GUIPkgInfo) error {
	if info.DesktopName == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, fsutil.DirMode); err != nil {
		return err
	}
	entry := fmt.Sprintf(desktopEntryFmt, info.DesktopName)
	return os.WriteFile(filepath.Join(dir, info.Name+".desktop"), []byte(entry), fileMode)
}

// Exec execs the GUI app with args appended after its own Args, erroring when
// it is not installed yet.
func Exec(args []string) error {
	return launch(false, args)
}

// Autostart execs the GUI app, waiting out the boot's install first — the
// session's XDG autostart lands here before the entrypoint's download finishes.
func Autostart() error {
	return launch(true, nil)
}

func launch(wait bool, args []string) error {
	info, bin, err := resolve()
	if err != nil {
		return err
	}
	if wait {
		waitInstalled(bin, func() { time.Sleep(pollInterval) })
		logLaunch(info.Name)
	} else if !installed(bin) {
		return fmt.Errorf("guiapp: %s is not installed at %s", info.Name, bin)
	}
	argv := append([]string{bin}, append(append([]string{}, info.Args...), args...)...)
	//nolint:gosec // the launcher's job: exec the app the run's env names
	return syscall.Exec(bin, argv, os.Environ())
}

// resolve returns the env's GUI app plus its executable's path under the apps
// volume: <AppsRoot>/<name>/current/<RelBin>.
func resolve() (pkginfo.GUIPkgInfo, string, error) {
	body := os.Getenv(pkginfo.GUIAppEnvVar)
	if body == "" {
		return pkginfo.GUIPkgInfo{}, "", fmt.Errorf("guiapp: %s is not set", pkginfo.GUIAppEnvVar)
	}
	info, err := pkginfo.GUIFromJSON(body)
	if err != nil {
		return pkginfo.GUIPkgInfo{}, "", fmt.Errorf("guiapp: %w", err)
	}
	p, err := pkger.ForPkgInfo(info.PkgInfo)
	if err != nil {
		return pkginfo.GUIPkgInfo{}, "", fmt.Errorf("guiapp: %w", err)
	}
	bin := filepath.Join(install.AppsRoot, info.Name, "current", p.RelBin())
	return info, bin, nil
}

// installed reports whether bin exists with an exec bit — the symlink chain can
// outlive its target on the shared volume, and only the stat catches it.
func installed(bin string) bool {
	info, err := os.Stat(bin)
	return err == nil && info.Mode()&0o111 != 0
}

// waitInstalled polls until bin is installed, sleeping between checks.
func waitInstalled(bin string, sleep func()) {
	for !installed(bin) {
		sleep()
	}
}

// logLaunch records the launch and keeps the app's streams on logPath across
// the exec, both best-effort — a missing log never blocks the launch.
func logLaunch(name string) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return
	}
	defer func() { log.WarnErr("gui app log close", f.Close()) }()
	if _, err := fmt.Fprintf(f, "launching %s\n", name); err != nil {
		log.WarnErr("gui app log write", err)
	}
	for _, fd := range []int{1, 2} {
		if err := dupStream(int(f.Fd()), fd); err != nil {
			log.WarnErr("gui app stream redirect", err)
		}
	}
}
