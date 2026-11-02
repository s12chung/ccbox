// Package stream delivers a stream's bytes to its views, waiting on a marker line.
package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Options configures the stream's follow: its views and its ready wait
type Options struct {
	Writers []io.Writer // the stream's views

	ReadyWait *ReadyWait
	OnEnd     func() error // runs once, at the stream's end or a failed open; nil runs nothing
}

// ReadyWait holds the marker and the screen to show while waiting for ready.
type ReadyWait struct {
	Marker string
	Screen io.Writer // nil shows nothing
}

func (o Options) end() error {
	if o.OnEnd == nil {
		return nil
	}
	return o.OnEnd()
}

// Follow opens the stream and follows it per the options — a failed open still runs
// OnEnd, its error joined with the open's.
func Follow(open func() (io.ReadCloser, error), o Options) error {
	stream, err := open()
	if err != nil {
		return errors.Join(err, o.end())
	}

	writers := slices.Clone(o.Writers)
	var wait *markerWait
	if o.ReadyWait != nil {
		wait = newMarkerWait(o.ReadyWait.Marker, o.ReadyWait.Screen)
		writers = append(writers, wait)
	}

	done := tee(stream, writers...)
	ended := make(chan error, 1)
	go func() { // the stream's teardown
		end := <-done
		ended <- end // ahead of the cleanup: the wait fails closed on the raw end
		log.WarnErr("close stream views", errors.Join(
			errs.Swallow(end, io.ErrClosedPipe, os.ErrClosed, net.ErrClosed, context.Canceled), o.end()))
	}()
	if wait == nil {
		return nil
	}
	return wait.await(ended)
}

// AwaitMarker follows the opened stream until a line equals marker, then stops the follow —
// failing closed with the last logs when the stream ends first
func AwaitMarker(stream io.ReadCloser, marker string, stop func()) error {
	wait := newMarkerWait(marker, nil)
	if err := wait.await(tee(stream, wait)); err != nil {
		return err
	}
	stop() // the marker came: break the copy's read
	return nil
}

// tee copies the stream's bytes to every sink, closing the stream once the copy ends —
// the copy is the stream's only reader: a stop breaks its read (a ctx cancel), never closes it.
func tee(stream io.ReadCloser, writers ...io.Writer) <-chan error {
	w := io.MultiWriter(writers...)
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(w, stream)
		done <- errors.Join(copyErr, stream.Close())
	}()
	return done
}

// markerWait watches a followed stream's lines for a marker, mirroring its bytes to a
// screen. The copy goroutine alone writes through it.
type markerWait struct {
	marker string
	screen io.Writer // nil shows nothing

	found chan struct{} // closed at the marker, never sent on — await parks on it, isFound peeks
	lines []string
	buf   []byte
}

func newMarkerWait(marker string, screen io.Writer) *markerWait {
	return &markerWait{marker: marker, screen: screen, found: make(chan struct{})}
}

// Write mirrors the chunk to the screen until the marker, then scans its lines —
// partial lines buffer until their newline lands
func (w *markerWait) Write(p []byte) (int, error) {
	if w.isFound() {
		return len(p), nil // still wired into the tee until the copy ends: drop the chunk
	}
	if w.screen != nil {
		if _, err := w.screen.Write(p); err != nil {
			return len(p), err
		}
	}
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		w.scan(line)
	}
}

// scan reports the marker's arrival — only ever once
func (w *markerWait) scan(line string) bool {
	if w.isFound() {
		return false
	}
	if line == w.marker {
		close(w.found)
		return true
	}
	w.lines = append(w.lines, line)
	if len(w.lines) > 10 { // err below includes them: keep it short
		w.lines = w.lines[1:]
	}
	return false
}

// isFound reports whether the marker came
func (w *markerWait) isFound() bool {
	select {
	case <-w.found:
		return true
	default:
		return false
	}
}

// await waits the marker against the stream's end, failing closed when it ends first
func (w *markerWait) await(ended <-chan error) error {
	select {
	case <-w.found:
		return nil
	case end := <-ended:
		return errors.Join(end, w.err())
	}
}

func (w *markerWait) err() error {
	return fmt.Errorf("the logs ended before %q came; last logs:\n%s",
		w.marker, strings.Join(w.lines, "\n"))
}
