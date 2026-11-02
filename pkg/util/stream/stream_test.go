package stream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const marker = "boot: ready"

func TestFollow_OpenFails(t *testing.T) {
	openErr := errors.New("open boom")
	onEnd, ended := newOnEnd()

	err := Follow(func() (io.ReadCloser, error) { return nil, openErr }, Options{OnEnd: onEnd})

	require.ErrorIs(t, err, openErr)
	require.Eventually(t, ended, 2*time.Second, 2*time.Millisecond,
		"the never-opened stream still runs the views' end")
}

func TestFollow_Marker(t *testing.T) {
	f := streamFile(t)
	screen := &syncBuffer{}
	onEnd, ended := newOnEnd()
	stream, endStream := openStream("boot", marker, "late")

	require.NoError(t, Follow(func() (io.ReadCloser, error) { return stream, nil }, Options{
		Writers: []io.Writer{f}, ReadyWait: &ReadyWait{Marker: marker, Screen: screen}, OnEnd: onEnd,
	}))

	// the frames land async: the file carrying "late" proves the tee ran past the marker
	require.Eventually(t, func() bool {
		body, err := os.ReadFile(f.Name())
		return err == nil && strings.Contains(string(body), "late")
	}, 2*time.Second, 2*time.Millisecond)
	endStream()
	require.Eventually(t, ended, 2*time.Second, 2*time.Millisecond,
		"the stream's end runs OnEnd")
	assert.Contains(t, screen.String(), "boot", "the wait view shows the boot live")
	assert.Contains(t, screen.String(), marker, "the marker itself shows before the view retires")
	assert.NotContains(t, screen.String(), "late", "the retired view frees the session's terminal")
	body, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Contains(t, string(body), "late", "the file keeps the stream past the marker")
}

func TestFollow_NoWait(t *testing.T) {
	f := streamFile(t)
	screen := &syncBuffer{}
	onEnd, ended := newOnEnd()
	stream, endStream := openStream("boot")

	require.NoError(t, Follow(func() (io.ReadCloser, error) { return stream, nil }, Options{
		Writers: []io.Writer{f, screen}, OnEnd: onEnd,
	}), "a waitless stream returns at once, its tee carrying on")

	require.Eventually(t, func() bool { return strings.Contains(screen.String(), "boot") },
		2*time.Second, 2*time.Millisecond, "the views stream")
	endStream()
	require.Eventually(t, ended, 2*time.Second, 2*time.Millisecond,
		"the stream's end runs OnEnd")
	assert.Contains(t, screen.String(), "boot", "the foreground view streams for the stream's life")
	body, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Contains(t, string(body), "boot")
}

func TestFollow_Exited(t *testing.T) {
	f := streamFile(t)
	onEnd, ended := newOnEnd()
	stream := exitedStream("line one", "line two")

	err := Follow(func() (io.ReadCloser, error) { return stream, nil },
		Options{Writers: []io.Writer{f}, ReadyWait: &ReadyWait{Marker: marker}, OnEnd: onEnd})

	require.ErrorContains(t, err, "ended before")
	require.ErrorContains(t, err, "line one")
	require.ErrorContains(t, err, "line two")
	require.Eventually(t, ended, 2*time.Second, 2*time.Millisecond,
		"the fail-closed end still runs OnEnd")
}

func TestAwaitMarker_Marker(t *testing.T) {
	stderr := captureErrOut(t)
	stream, endStream := openStream("seeded", marker)

	stops := 0
	require.NoError(t, AwaitMarker(stream, marker, func() {
		stops++
		endStream() // the cancel's break: the copy's blocked read fails
	}))

	// the teardown ends without the stream's end: prompt, quiet, and single
	require.Equal(t, 1, stops)
	requireStreamClosed(t, stream)
	require.NoError(t, stream.overlapErr())
	assert.Empty(t, stderr.String(), "the marker's early return is not a failure to log")
}

func TestAwaitMarker_Exited(t *testing.T) {
	stderr := captureErrOut(t)
	stream := exitedStream("line one", "line two")

	err := AwaitMarker(stream, marker, func() { t.Error("an ended stream's wait must not stop it") })

	// the stream's last words name its death
	require.ErrorContains(t, err, "ended before")
	require.ErrorContains(t, err, "line one")
	require.ErrorContains(t, err, "line two")

	// an ended stream closes quietly
	requireStreamClosed(t, stream)
	require.NoError(t, stream.overlapErr())
	assert.Empty(t, stderr.String())
}

func TestAwaitMarker_StreamError(t *testing.T) {
	// the stream's read fails: the real cause surfaces with the fail-closed error
	stream := &followStream{readErr: errors.New("stream boom")}
	err := AwaitMarker(stream, marker, func() { t.Error("a failed stream's wait must not stop it") })
	require.ErrorContains(t, err, "ended before")
	require.ErrorContains(t, err, "stream boom")

	requireStreamClosed(t, stream)
}

func TestMarkerWait_Write(t *testing.T) {
	screen := &syncBuffer{}
	wait := newMarkerWait(marker, screen)

	n, err := wait.Write([]byte("one\ntw"))
	require.NoError(t, err)
	assert.Equal(t, len("one\ntw"), n)
	_, err = wait.Write([]byte("o\n"))
	require.NoError(t, err)
	_, err = wait.Write([]byte(marker + "\n"))
	require.NoError(t, err)
	n, err = wait.Write([]byte("late\n"))
	require.NoError(t, err)

	assert.Equal(t, len("late\n"), n, "the retired screen swallows its writes whole")
	assert.Equal(t, "one\ntwo\n"+marker+"\n", screen.String(),
		"the screen mirrors up to and including the marker's chunk")
	err = wait.err()
	require.ErrorContains(t, err, "two", "partial lines buffer until their newline lands")
	assert.NotContains(t, err.Error(), "late", "the seen wait scans on inertly")
}

func TestMarkerWait_Scan(t *testing.T) {
	wait := newMarkerWait(marker, nil)

	assert.False(t, wait.scan("booting"))
	assert.True(t, wait.scan(marker), "the marker arrives")
	assert.False(t, wait.scan(marker), "only ever once")
	assert.False(t, wait.scan("a late log line"), "the seen wait scans on inertly")
	select {
	case <-wait.found:
	default:
		t.Fatal("the marker's arrival fires Found")
	}
}

func TestMarkerWait_Err_Truncates(t *testing.T) {
	wait := &markerWait{marker: marker}
	for i := range 12 {
		assert.False(t, wait.scan(fmt.Sprintf("boot %02d", i)))
	}

	err := wait.err()

	require.ErrorContains(t, err, "ended before")
	require.ErrorContains(t, err, "boot 11")
	assert.NotContains(t, err.Error(), "boot 01", "the error keeps only the last 10 lines")
}

// newOnEnd returns an Options.OnEnd recording its run, and its ran probe
func newOnEnd() (func() error, func() bool) {
	finished := make(chan struct{}, 1)
	return func() error { finished <- struct{}{}; return nil },
		func() bool {
			select {
			case <-finished:
				return true
			default:
				return false
			}
		}
}

// streamFile is a stream view's temp log file, closed by the test's end
func streamFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stream")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// openStream fakes a followed stream over lines written in a goroutine; it stays open
// like a live container's until endStream ends it.
func openStream(lines ...string) (*followStream, func()) {
	pipeR, pipeW := io.Pipe()
	go func() {
		for _, line := range lines {
			_, _ = pipeW.Write([]byte(line + "\n"))
		}
	}()
	return &followStream{readCloser: pipeR}, sync.OnceFunc(func() { _ = pipeW.Close() })
}

// captureErrOut swaps in a concurrency-safe warn/error buffer, restored at cleanup
func captureErrOut(t *testing.T) *syncBuffer {
	t.Helper()
	stderr := &syncBuffer{}
	t.Cleanup(log.SwapErrWriter(stderr))
	return stderr
}

// syncBuffer is a race-safe bytes.Buffer, polled while the copy goroutine writes
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// requireStreamClosed waits for the teardown to close the stream exactly once
func requireStreamClosed(t *testing.T, stream *followStream) {
	t.Helper()
	require.Eventually(t, func() bool { return stream.closeCount() == 1 }, 2*time.Second, 2*time.Millisecond)
}

// exitedStream is openStream ended right after the lines: an exiting container.
func exitedStream(lines ...string) *followStream {
	pipeR, pipeW := io.Pipe()
	go func() {
		for _, line := range lines {
			_, _ = pipeW.Write([]byte(line + "\n"))
		}
		_ = pipeW.Close()
	}()
	return &followStream{readCloser: pipeR}
}

// followStream fakes a followed stream, naming any close over an in-flight or later read
type followStream struct {
	readCloser io.ReadCloser
	readErr    error // the stream's read failure, set at construction

	mu       sync.Mutex
	inRead   int
	closes   int
	isClosed bool
	overlap  error
}

func (s *followStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	switch {
	case s.isClosed:
		s.overlap = errors.New("read after close")
		s.mu.Unlock()
		return 0, io.ErrClosedPipe
	case s.readErr != nil:
		readErr := s.readErr
		s.mu.Unlock()
		return 0, readErr
	}
	s.inRead++
	s.mu.Unlock()

	n, err := s.readCloser.Read(p)

	s.mu.Lock()
	s.inRead--
	s.mu.Unlock()
	return n, err
}

func (s *followStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inRead > 0 {
		s.overlap = errors.New("close over an in-flight read")
	}
	if s.isClosed {
		return nil
	}
	s.isClosed = true
	s.closes++
	if s.readCloser == nil {
		return nil
	}
	return s.readCloser.Close()
}

func (s *followStream) closeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closes
}

func (s *followStream) overlapErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overlap
}
