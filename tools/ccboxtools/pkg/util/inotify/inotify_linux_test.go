//go:build linux

package inotify

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatch(t *testing.T) {
	dir := t.TempDir()
	changed := make(chan string, 8)
	require.NoError(t, watch(context.Background(), dir, func(fileName string) { changed <- fileName }))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "allow.txt"), []byte("x"), 0o600))

	select {
	case fileName := <-changed:
		assert.Equal(t, "allow.txt", fileName)
	case <-time.After(5 * time.Second):
		t.Fatal("the watch never reported the write")
	}
}

func TestWatchFile_FiltersOtherNames(t *testing.T) {
	dir := t.TempDir()
	changed := make(chan struct{}, 8)
	require.NoError(t, WatchFile(context.Background(), dir, "allow.txt", func() { changed <- struct{}{} }))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o600))
	select {
	case <-changed:
		t.Fatal("another file's change must not call back")
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "allow.txt"), []byte("x"), 0o600))
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch never reported the watched file's write")
	}
}

func TestWatchFile_ContextCancelStops(t *testing.T) {
	dir := t.TempDir()
	changed := make(chan struct{}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, WatchFile(ctx, dir, "allow.txt", func() { changed <- struct{}{} }))
	cancel()

	time.Sleep(3 * pollInterval) // the read loop exits within one poll
	require.NoError(t, os.WriteFile(filepath.Join(dir, "allow.txt"), []byte("x"), 0o600))
	select {
	case <-changed:
		t.Fatal("the cancelled watch must not call back")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestEventNames(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want []string
	}{
		{"single", eventBytes("allow.txt"), []string{"allow.txt"}},
		{"two", append(eventBytes("allow.txt"), eventBytes("other")...), []string{"allow.txt", "other"}},
		{"truncated tail", eventBytes("allow.txt")[:syscall.SizeofInotifyEvent+3], nil},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, eventNames(tt.buf))
		})
	}
}

// eventBytes renders one kernel-shaped inotify event carrying name.
func eventBytes(name string) []byte {
	nameLen := (len(name) + 1 + 3) &^ 3 // nul plus the kernel's 4-byte alignment
	buf := make([]byte, syscall.SizeofInotifyEvent+nameLen)
	ev := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[0])) //nolint:gosec // the kernel's own fixed-layout event
	ev.Mask = writeMask
	ev.Len = uint32(nameLen) //nolint:gosec // G115: a test-built name, far under 2^32
	copy(buf[syscall.SizeofInotifyEvent:], name+"\x00")
	return buf
}
