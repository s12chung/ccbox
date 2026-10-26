//go:build linux

package inotify

import (
	"bytes"
	"context"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// writeMask catches every way an atomic file write can land in the watched
	// dir: written in place, renamed onto, or created fresh.
	writeMask = syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO | syscall.IN_CREATE

	// pollInterval paces the read loop's waits — the fd is non-blocking so ctx
	// can end the loop, at the cost of this much event latency.
	pollInterval = 250 * time.Millisecond
)

// watch watches dir for content changes, reporting each event's file name to
// changed, until ctx ends.
func watch(ctx context.Context, dir string, changed func(fileName string)) error {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return fmt.Errorf("inotify: %w", err)
	}
	if _, err := syscall.InotifyAddWatch(fd, dir, writeMask); err != nil {
		return fmt.Errorf("inotify watch %s: %w", dir, err)
	}
	go readLoop(ctx, dir, fd, changed)
	return nil
}

// readLoop reads events until ctx ends — without events, the non-blocking
// read falls through to a ctx-aware wait.
func readLoop(ctx context.Context, dir string, fd int, changed func(fileName string)) {
	defer func() { log.WarnErr("close inotify fd", syscall.Close(fd)) }()
	buf := make([]byte, 4096)
	for {
		n, err := syscall.Read(fd, buf)
		switch {
		case err == syscall.EAGAIN, err == syscall.EINTR:
		case err != nil:
			log.Warnf("watch on %s ended: %v", dir, err)
			return
		case n == 0:
			return
		default:
			for _, fileName := range eventNames(buf[:n]) {
				changed(fileName)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

// eventNames reads the changed paths out of one inotify read's event buffer.
func eventNames(buf []byte) []string {
	var names []string
	for len(buf) >= syscall.SizeofInotifyEvent {
		ev := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[0])) //nolint:gosec // the kernel's own fixed-layout event
		end := syscall.SizeofInotifyEvent + int(ev.Len)
		if len(buf) < end {
			break
		}
		name := buf[syscall.SizeofInotifyEvent:end]
		if before, _, found := bytes.Cut(name, []byte{0}); found {
			names = append(names, string(before))
		}
		buf = buf[end:]
	}
	return names
}
