package entrypoint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleMountinfo is a mountinfo(5) excerpt: sysfs, proc, and a read-only bind
// of the git config dir (fields: id parent major:minor root mount-point opts ...).
const sampleMountinfo = `18 24 0:17 / /sys rw,nosuid,nodev,noexec,relatime shared:6 - sysfs none rw
19 24 0:18 / /proc rw,nosuid,nodev,noexec,relatime shared:7 - procfs none rw
39 24 253:1 /exports/git /home/ccbox/.config/git ro,relatime shared:20 - ext4 /dev/sda1 rw,relatime
`

// rwMountinfo shadows sampleMountinfo's ro bind with a later rw entry.
const rwMountinfo = sampleMountinfo + "40 24 253:1 /exports/git /home/ccbox/.config/git rw,relatime - ext4 /dev/sda1 rw,relatime\n"

func TestMountOptsFor(t *testing.T) {
	tests := []struct {
		name      string
		mountinfo string
		path      string
		wantOpts  string
		wantOK    bool
	}{
		{"match", sampleMountinfo, gitConfigMount, "ro,relatime", true},
		{"last match wins", rwMountinfo, gitConfigMount, "rw,relatime", true},
		{"missing", sampleMountinfo, "/home/ccbox/other", "", false},
		{"short lines", "not a mountinfo line\n", gitConfigMount, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, ok := mountOptsFor(tt.mountinfo, tt.path)

			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantOpts, opts)
		})
	}
}

func TestCheckMountRO(t *testing.T) {
	tests := []struct {
		name      string
		mountinfo string
		path      string
		wantErr   string
	}{
		{"ok", sampleMountinfo, gitConfigMount, ""},
		{"rw", rwMountinfo, gitConfigMount, "not a read-only mount"},
		{"no entry", sampleMountinfo, "/home/ccbox/other", "no mount entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkMountRO(tt.mountinfo, tt.path)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCheckGitMountRO(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	// the mountinfo source is a fixture — /proc/self/mountinfo is Linux-only
	mountinfoFile := filepath.Join(t.TempDir(), "mountinfo")

	roDir := t.TempDir()
	roEntry := fmt.Sprintf("39 24 253:1 /exports/git %s ro,relatime shared:20 - ext4 /dev/sda1 rw,relatime\n", roDir)

	tests := []struct {
		name          string
		path          string
		mountinfoBody string
		wantErr       string
	}{
		{"absent, so no bind from host", filepath.Join(t.TempDir(), "missing"), "", ""},
		{"not a dir, so not a gitconfig from host", file, "", ""},
		{"dir without mount", t.TempDir(), sampleMountinfo, "no mount entry"},
		{"dir mounted ro", roDir, roEntry, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mountinfoBody != "" {
				require.NoError(t, os.WriteFile(mountinfoFile, []byte(tt.mountinfoBody), 0o600))
			}

			err := checkGitMountRO(tt.path, mountinfoFile)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
