// Package inotify watches a directory through the kernel's inotify: the watch
// setup, the event read loop, and the wire format's parsing — file names only.
package inotify

import "context"

// WatchFile watches name within dir for content changes. The dir carries the
// watch: a rewrite replaces the file's inode, not its name, and inotify
// watches inodes. Setup fails synchronously; the read loop runs until ctx ends.
func WatchFile(ctx context.Context, dir, name string, changed func()) error {
	return watch(ctx, dir, func(fileName string) {
		if fileName == name {
			changed()
		}
	})
}
