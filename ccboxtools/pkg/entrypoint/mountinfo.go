package entrypoint

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// gitConfigMount is the host global git dir, bind-mounted read-only at git's
// default XDG path.
const gitConfigMount = "/home/ccbox/.config/git"

const linuxMountinfoPath = "/proc/self/mountinfo"

// checkGitMountRO asserts the git config dir is a read-only mount
func checkGitMountRO(path, mountinfoPath string) error {
	fi, err := os.Stat(path)
	// absent, so no bind from host OR not a dir, so not a gitconfig from host
	if err != nil || !fi.IsDir() {
		return nil // nolint:nilerr // explained above
	}
	//nolint:gosec // mountinfoPath is the package const at the call site; the param is test injection
	body, err := os.ReadFile(mountinfoPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", mountinfoPath, err)
	}
	return checkMountRO(string(body), path)
}

// checkMountRO verifies path carries a mountinfo entry mounted read-only.
func checkMountRO(mountinfo, path string) error {
	opts, ok := mountOptsFor(mountinfo, path)
	if !ok {
		return fmt.Errorf("%s has no mount entry in %s", path, linuxMountinfoPath)
	}
	if !slices.Contains(strings.Split(opts, ","), "ro") {
		return fmt.Errorf("%s is not a read-only mount (opts: %s)", path, opts)
	}
	return nil
}

// mountOptsFor returns path's mount options — field 6 in mountinfo(5) — from the
// last entry whose mount point (field 5) is path, mirroring the awk this replaces.
func mountOptsFor(mountinfo, path string) (string, bool) {
	opts := ""
	found := false
	for line := range strings.SplitSeq(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[4] != path {
			continue
		}
		opts, found = fields[5], true
	}
	return opts, found
}
