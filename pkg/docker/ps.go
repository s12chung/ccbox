package docker

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// DevboxStatus is one project devbox's live session state
type DevboxStatus struct {
	Name       string // the devbox's fixed container name
	State      string // the engine's own word: running, exited, ...
	Execs      int    // the devbox's live exec sessions
	Foreground bool   // a foreground run streams the devbox's logs
}

// DevboxStatuses lists the project's devboxes (every variant) with their live
// session state, name-ascending — none yet, an empty list
func DevboxStatuses(ctxD *dock.CtxD, projectDir string) ([]DevboxStatus, error) {
	list, err := ctxD.D.ContainerList(ctxD.Ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", ccboxProjectLabelKey+"="+projectDir)),
	})
	if err != nil {
		return nil, err
	}

	statuses := make([]DevboxStatus, 0, len(list))
	for _, c := range list {
		status, err := devboxStatus(c)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	slices.SortFunc(statuses, func(a, b DevboxStatus) int { return strings.Compare(a.Name, b.Name) })
	return statuses, nil
}

// HoldersDir is the devbox's holder flock dir on the host, keyed by its name
func HoldersDir(name string) string { return filepath.Join(userdir.Tmp(), "devbox-holders", name) }

// devboxStatus reads a listed devbox's live session state — a stopped devbox
// has no live sessions to count
func devboxStatus(c container.Summary) (DevboxStatus, error) {
	if len(c.Names) == 0 {
		return DevboxStatus{}, errors.New("docker: a listed devbox carries no name")
	}

	name := strings.TrimPrefix(c.Names[0], "/") // the engine prefixes container names with /
	s := DevboxStatus{Name: name, State: c.State}
	if c.State != "running" {
		return s, nil
	}

	release, holders, err := flock.MultiFlock{Dir: HoldersDir(name)}.Clean()
	if err != nil {
		return s, err
	}
	defer log.Defer("release the holders dir lock", release)
	if s.Foreground, err = isForegrounded(foregroundMarkPath(HoldersDir(name))); err != nil {
		return s, err
	}

	s.Execs = holders
	if s.Foreground {
		s.Execs-- // the foreground run is itself a holder
	}
	return s, nil
}

// isForegrounded reports whether a foreground run holds the devbox's mark
func isForegrounded(markPath string) (bool, error) {
	mark, err := flock.TryEx(markPath)
	if errors.Is(err, flock.ErrHeld) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer log.Defer("release the foreground probe", mark.Release)
	return false, nil
}
