package sharer

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// The tests pin the driver and its axes on two wirings — a file+symlink share in the
// AGENTS-doc share's shape, and a dir+direct share in the persist share's — under a
// temp userDir; the shared flows run over both via wiring. The domain packages pin
// their own wiring through Begin.

// testMount is the file bind's container path, as dmap passes it in
const testMount = "/home/ccbox/.ccbox/tmp/agents/claude/doc.md"

// fileShare wires a file+symlink Share, binding source
func fileShare(source func() ([]byte, error)) Share {
	return Share{
		Name:     "test",
		RealPath: realFile(),
		Content:  FileScratch{Source: source},
		Sync:     SymlinkScratch{Mount: testMount},
	}
}

// sharedSource is the fixed body the file bind holds, as a seeded doc would
func sharedSource() ([]byte, error) { return []byte("shared"), nil }

// realFile is the file share's persistent path: ~/.ccbox/claude/doc.md
func realFile() string { return filepath.Join(userdir.Dir(), "claude", "doc.md") }

// bindFile is the file bound while realFile is missing
func bindFile() string { return filepath.Join(userdir.Tmp(), "agents", "claude", "doc.md") }

// baseFile is the baseline the bind is diffed against
func baseFile() string { return bindFile() + ".orig" }

// dirSeedFS is the dir scratch's embedded tree
var dirSeedFS = fstest.MapFS{"README.md": &fstest.MapFile{Data: []byte("seeded")}}

// dirShare wires a dir+direct Share, seeded from dirSeedFS
func dirShare() Share {
	return Share{
		Name:     "test",
		RealPath: dirReal(),
		Content:  DirScratch{SeedFS: dirSeedFS},
		Sync:     DirectScratch{},
	}
}

// dirReal is the dir share's persistent path: ~/.ccbox/state
func dirReal() string { return filepath.Join(userdir.Dir(), "state") }

// dirScratch is the dir bound while dirReal is missing: the same rel path under userdir.Tmp()
func dirScratch() string { return filepath.Join(userdir.Tmp(), "state") }

// wiring is one of the two wirings the shared flows run on, across the ScratchContent and
// ScratchSync axes: the file+symlink side in the AGENTS-doc share's shape, the dir+direct
// side in the persist's. dir toggles the side; every step derives from it.
type wiring struct {
	name string
	dir  bool
}

var (
	// symlinkWiring, the file+symlink side
	symlinkWiring = wiring{name: "Symlink"}
	// directWiring, the dir+direct side
	directWiring = wiring{name: "Direct", dir: true}
	// wirings, the sides each shared flow runs over
	wirings = []wiring{symlinkWiring, directWiring}
)

// newShare is the wiring's share
func (w wiring) newShare() Share {
	if w.dir {
		return dirShare()
	}
	return fileShare(sharedSource)
}

// bind is the host path bound while Real is missing
func (w wiring) bind() string {
	if w.dir {
		return dirScratch()
	}
	return bindFile()
}

// lateBind is the bind once Real is occupied: Real itself, or nothing — Real is
// reached via its own bind elsewhere
func (w wiring) lateBind() string {
	if w.dir {
		return dirReal()
	}
	return ""
}

// edit writes body into the bound file, as the run would
func (w wiring) edit(t *testing.T, body string) {
	t.Helper()
	if w.dir {
		writeScratch(t, "notes.txt", body)
		return
	}
	writeFile(t, bindFile(), body)
}

// occupy writes the host's own Real mid-session
func (w wiring) occupy(t *testing.T, body string) {
	t.Helper()
	if w.dir {
		writeFile(t, filepath.Join(dirReal(), "host.txt"), body)
		return
	}
	require.NoError(t, os.RemoveAll(realFile())) // drops our symlink, if any
	writeFile(t, realFile(), body)
}

// assertBound asserts the bound file holds the run's edit
func (w wiring) assertBound(t *testing.T, body string) {
	t.Helper()
	if w.dir {
		assertFile(t, filepath.Join(dirScratch(), "notes.txt"), body)
		return
	}
	assert.Equal(t, body, readFile(t, bindFile()))
}

// assertPromoted asserts Real holds the run's promoted body
func (w wiring) assertPromoted(t *testing.T, body string) {
	t.Helper()
	if w.dir {
		assertSeeded(t, filepath.Join(dirReal(), "README.md"))
		assertFile(t, filepath.Join(dirReal(), "notes.txt"), body)
		return
	}
	assert.Equal(t, body, readFile(t, realFile()))
	assertRegular(t, realFile())
}

// assertStands asserts the host's own Real body stands, promoted over by nothing
func (w wiring) assertStands(t *testing.T, body string) {
	t.Helper()
	if w.dir {
		assertFile(t, filepath.Join(dirReal(), "host.txt"), body)
		return
	}
	w.assertPromoted(t, body)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) // #nosec G304 -- the test's own path
	require.NoError(t, err)
	return string(body)
}

func gone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// present is gone's opposite
func present(t *testing.T, path string) bool {
	t.Helper()
	return !gone(t, path)
}

// lstatGone is gone without following symlinks
func lstatGone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

// linkFile creates the share's symlink at path, pointing at target
func linkFile(t *testing.T, path, target string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
	require.NoError(t, os.Symlink(target, path))
}

// assertSymlinked asserts realFile is the share's own symlink into the mount
func assertSymlinked(t *testing.T) {
	t.Helper()
	info, err := os.Lstat(realFile())
	require.NoError(t, err)
	require.NotEqual(t, 0, info.Mode()&os.ModeSymlink, "a symlink, not a regular file")
	target, err := os.Readlink(realFile())
	require.NoError(t, err)
	assert.Equal(t, testMount, target)
}

// assertRegular asserts path is a regular file, not a symlink
func assertRegular(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "a regular file, not a symlink")
}

// writeScratch writes body at rel under the dir scratch
func writeScratch(t *testing.T, rel, body string) {
	t.Helper()
	writeFile(t, filepath.Join(dirScratch(), rel), body)
}

func assertSeeded(t *testing.T, path string) {
	t.Helper()
	seedBody, err := fs.ReadFile(dirSeedFS, "README.md")
	require.NoError(t, err)
	assertFile(t, path, string(seedBody))
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) // #nosec G304 -- reads the test's own path
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}

func TestDirect_ScratchPath(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	// Real's rel path re-rooted under userdir.Tmp()
	assert.Equal(t, "/home/me/.ccbox/tmp/persist/-Users-me-proj",
		DirectScratch{}.ScratchPath("/home/me/.ccbox/persist/-Users-me-proj"))
}

// TestSymlink_Exists: an occupation is ours only when Real is absent or our own
// symlink — everything else on Real stays owned, untouched
func TestSymlink_Exists(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T)
		owned    bool
	}{
		{
			caseName: "MissingReShares",
			setup:    func(*testing.T) {},
		},
		{
			caseName: "EmptyFileIsOwned", // an empty doc is the user's own, not a missing one
			setup:    func(t *testing.T) { writeFile(t, realFile(), "") },
			owned:    true,
		},
		{
			caseName: "ForeignSymlinkIsOwned", // e.g. the user's dotfiles repo
			setup:    func(t *testing.T) { linkFile(t, realFile(), "/elsewhere/doc.md") },
			owned:    true,
		},
		{
			caseName: "OursDanglingReShares", // ours: dangling on the host by design
			setup:    func(t *testing.T) { linkFile(t, realFile(), testMount) },
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			l := SymlinkScratch{Mount: testMount}

			tc.setup(t)
			assert.Equal(t, tc.owned, l.RealPresent(realFile()))
		})
	}
}

// TestShare_VerifyShared: a live entry implies the share is present — the scratch's
// bind, or Real itself once occupied
func TestShare_VerifyShared(t *testing.T) {
	for _, w := range wirings {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := w.newShare()

			require.Error(t, s.verifyShared(), "nothing present: neither the bind nor Real")

			w.occupy(t, "own doc")
			require.NoError(t, s.verifyShared(), "the occupied Real is the share")
		})
	}
}

func TestShare_Begin_Piggyback(t *testing.T) {
	for _, w := range wirings {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := w.newShare()

			// a live run binds the share and edits it
			_, clean, err := s.Begin()
			require.NoError(t, err)
			w.edit(t, "memory")

			// a second run piggybacks: the same bind, the live run's edit intact
			bind, piggybackClean, err := s.Begin()
			require.NoError(t, err)
			assert.Equal(t, w.bind(), bind)
			w.assertBound(t, "memory")

			// the piggyback's cleanup leaves the share for the live run
			require.NoError(t, piggybackClean())
			assert.True(t, present(t, w.bind()))

			// the last run out promotes and removes the share
			require.NoError(t, clean())
			w.assertPromoted(t, "memory")
			assert.True(t, gone(t, w.bind()))
		})
	}
}

// TestShare_Begin_LatecomerBindsReal: a latecomer joining a live session whose host
// has written its own Real mid-session binds Real, and the live run's share is
// left alone
func TestShare_Begin_LatecomerBindsReal(t *testing.T) {
	for _, w := range wirings {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := w.newShare()

			// a live run binds the share
			_, clean, err := s.Begin()
			require.NoError(t, err)

			// the host writes its own Real mid-session
			w.occupy(t, "own doc")

			// the latecomer binds Real, not the scratch
			bind, piggybackClean, err := s.Begin()
			require.NoError(t, err)
			assert.Equal(t, w.lateBind(), bind)

			// the live run's share is untouched until its last out cleans up
			require.NoError(t, piggybackClean())
			assert.True(t, present(t, w.bind()))

			// the last out promotes nothing: the scratch was never edited
			require.NoError(t, clean())
			w.assertStands(t, "own doc")
			assert.True(t, gone(t, w.bind()))
		})
	}
}

// TestShare_Begin_JoinsOccupiedReal: the first run binds Real's own occupation
// directly, no scratch written; the latecomer joins it all the same
func TestShare_Begin_JoinsOccupiedReal(t *testing.T) {
	for _, w := range wirings {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := w.newShare()
			w.occupy(t, "own doc")

			bind, clean, err := s.Begin()
			require.NoError(t, err)
			assert.Equal(t, w.lateBind(), bind)
			assert.True(t, gone(t, w.bind()), "an existing Real binds directly, no scratch written")

			lateBind, piggybackClean, err := s.Begin()
			require.NoError(t, err)
			assert.Equal(t, w.lateBind(), lateBind)
			w.assertStands(t, "own doc")

			require.NoError(t, piggybackClean())
			require.NoError(t, clean())
			assert.True(t, gone(t, w.bind()))
		})
	}
}

// TestShare_Begin_CleansLeftover: a crashed run's leftovers are cleaned before the
// share re-binds — a changed leftover promotes, an unchanged one re-shares
func TestShare_Begin_CleansLeftover(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T) // the leftover state
		binds    bool               // whether the share re-binds a scratch
		real     *string            // the expected realFile body after; nil when nothing promoted
	}{
		{
			caseName: "PromotesChanged",
			setup: func(t *testing.T) {
				writeFile(t, bindFile(), "edited") // no baseline: diffed against the source
			},
			real: new("edited"),
		},
		{
			caseName: "ResharesUnchanged",
			setup: func(t *testing.T) {
				writeFile(t, bindFile(), "shared") // unchanged from its source
			},
			binds: true,
		},
		{
			caseName: "ResharesUnderOursOccupation",
			setup: func(t *testing.T) {
				linkFile(t, realFile(), testMount) // our leftover symlink, its scratch gone
			},
			binds: true,
		},
		{
			// a crash between the promote and the cleanup's removals
			caseName: "AfterPromote",
			setup: func(t *testing.T) {
				writeFile(t, bindFile(), "edited")
				writeFile(t, baseFile(), "shared")
				writeFile(t, realFile(), "edited") // the promoted edit
			},
			real: new("edited"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := fileShare(sharedSource)
			tc.setup(t)

			bind, cleanup, err := s.Begin()
			require.NoError(t, err)

			if tc.binds {
				assert.Equal(t, bindFile(), bind)
				assert.Equal(t, "shared", readFile(t, bindFile()))
				assertSymlinked(t)
			} else {
				assert.Empty(t, bind)
			}
			if tc.real != nil {
				assert.Equal(t, *tc.real, readFile(t, realFile()))
				assertRegular(t, realFile())
			}

			require.NoError(t, cleanup())
			assert.True(t, gone(t, bindFile()))
			assert.True(t, gone(t, baseFile()))
			if tc.real == nil {
				assert.True(t, lstatGone(t, realFile()), "nothing promoted; our symlink is dropped")
			}
		})
	}
}

// TestShare_Begin_CleansLeftover_IntoExistingReal: a crashed run's changed leftover
// promotes into an existing Real, its differing copies kept aside; an unchanged one
// is swept, Real untouched
func TestShare_Begin_CleansLeftover_IntoExistingReal(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		leftover func(t *testing.T) // the crashed run's dir leftover
		expected func(t *testing.T) // Real after the next run's clean
	}{
		{
			// the crashed run changed its scratch before dying
			caseName: "PromotesChanged",
			leftover: func(t *testing.T) {
				writeScratch(t, "README.md", "run's edit")
				writeScratch(t, "notes.txt", "written before the crash")
			},
			expected: func(t *testing.T) {
				assertFile(t, filepath.Join(dirReal(), "README.md"), "run's edit")
				assertFile(t, filepath.Join(dirReal(), "notes.txt"), "written before the crash")
				variants, err := filepath.Glob(filepath.Join(dirReal(), "README.run-*.md"))
				require.NoError(t, err)
				require.Len(t, variants, 1)
				assertFile(t, variants[0], "real's own")
			},
		},
		{
			// the crashed run never touched its seeded scratch
			caseName: "SweepsUnchanged",
			leftover: func(t *testing.T) {
				seedBody, err := fs.ReadFile(dirSeedFS, "README.md")
				require.NoError(t, err)
				writeScratch(t, "README.md", string(seedBody))
			},
			expected: func(t *testing.T) {
				assertFile(t, filepath.Join(dirReal(), "README.md"), "real's own")
				variants, err := filepath.Glob(filepath.Join(dirReal(), "*.run-*"))
				require.NoError(t, err)
				assert.Empty(t, variants, "an unchanged leftover promotes nothing")
			},
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			tc.leftover(t)

			// Real exists from an earlier clean, holding its own copy
			require.NoError(t, os.MkdirAll(dirReal(), ioutil.Dir))
			require.NoError(t, os.WriteFile(filepath.Join(dirReal(), "README.md"), []byte("real's own"), ioutil.File))

			// the next Begin promotes the leftover and binds Real
			bind, clean, err := dirShare().Begin()
			require.NoError(t, err)
			assert.Equal(t, dirReal(), bind)
			require.NoError(t, clean())

			tc.expected(t)
			assert.NoDirExists(t, dirScratch())
		})
	}
}

// TestShare_Begin_AfterCrashedRun: a crashed run's registry entry is released by the
// kernel; the next run re-shares the seeded scratch, and its changes promote
func TestShare_Begin_AfterCrashedRun(t *testing.T) {
	for _, w := range wirings {
		t.Run(w.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			crashRun(t, w)
			assert.True(t, present(t, w.bind()), "the crashed run's seeded scratch")

			bind, clean, err := w.newShare().Begin()
			require.NoError(t, err)
			assert.Equal(t, w.bind(), bind, "the unchanged leftover re-shares")

			w.edit(t, "edit after the crash")
			require.NoError(t, clean())
			w.assertPromoted(t, "edit after the crash")
			assert.True(t, gone(t, w.bind()))
		})
	}
}

// TestShare_Cleanup cleans up the bound scratch: a diff is promoted as-is; an owned
// Real is a no-op
func TestShare_Cleanup(t *testing.T) {
	for _, tc := range []struct {
		caseName     string
		setup        func(t *testing.T) // written before the run
		bindEdit     string             // written to the bound file during the run; "" leaves it
		expectedReal *string            // the expected realFile body after; nil when nothing promoted
	}{
		{
			caseName:     "PromotesChanged",
			bindEdit:     "memory",
			expectedReal: new("memory"),
		},
		{
			caseName: "KeepsOriginalWhenUnchanged",
		},
		{
			caseName:     "NoopWhenNotBound",
			setup:        func(t *testing.T) { writeFile(t, realFile(), "own doc") },
			expectedReal: new("own doc"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			if tc.setup != nil {
				tc.setup(t)
			}
			s := fileShare(sharedSource)

			_, cleanup, err := s.Begin()
			require.NoError(t, err)
			if tc.bindEdit != "" {
				writeFile(t, bindFile(), tc.bindEdit)
			}

			require.NoError(t, cleanup())

			if tc.expectedReal == nil {
				assert.True(t, lstatGone(t, realFile()), "nothing promoted; our symlink is dropped")
			} else {
				assert.Equal(t, *tc.expectedReal, readFile(t, realFile()))
				assertRegular(t, realFile())
			}
			assert.True(t, gone(t, bindFile()), "scratch removed")
			assert.True(t, gone(t, baseFile()), "baseline removed")
		})
	}
}

func TestShare_Cleanup_KeepsScratchOnPromoteError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := fileShare(sharedSource)

	_, cleanup, err := s.Begin()
	require.NoError(t, err)
	writeFile(t, bindFile(), "memory")
	require.NoError(t, os.Remove(realFile()))               // clear our symlink
	require.NoError(t, os.MkdirAll(realFile(), ioutil.Dir)) // a directory is unreadable: promotion fails

	require.Error(t, cleanup(), "Real is a directory")
	assert.Equal(t, "memory", readFile(t, bindFile()), "scratch kept for the next run")
	assert.Equal(t, "shared", readFile(t, baseFile()), "baseline kept with the scratch")
}

func TestShare_Cleanup_LoneSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := Share{Name: "test", RealPath: realFile(), Sync: SymlinkScratch{Mount: testMount}}

	linkFile(t, realFile(), testMount) // leftover symlink: its scratch is already gone

	require.NoError(t, s.clean())

	assert.True(t, lstatGone(t, realFile()), "the lone symlink is dropped")
}

func TestShare_Cleanup_DiffErrorDropsSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	linkFile(t, realFile(), testMount)
	writeFile(t, bindFile(), "leftover")
	s := fileShare(func() ([]byte, error) { return nil, errors.New("no source doc") })

	require.Error(t, s.clean())

	assert.True(t, lstatGone(t, realFile()), "the symlink is dropped despite the failed clean")
	assert.False(t, gone(t, bindFile()), "the scratch is kept for the next run")
}

func TestShare_Cleanup_LoneBaseline(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := Share{Name: "test", RealPath: realFile(), Sync: SymlinkScratch{Mount: testMount}}

	// a crash left its baseline; the scratch is gone
	writeFile(t, baseFile(), "doc")

	// the lone baseline is swept with the scratch's root
	require.NoError(t, s.clean())
	assert.True(t, gone(t, baseFile()))
}

// TestShare_Cleanup_HostChangedDuringRun: the source's host edits promote nothing —
// the diff runs against the baseline, not the source; the bind's edits still promote
func TestShare_Cleanup_HostChangedDuringRun(t *testing.T) {
	for _, tc := range []struct {
		caseName     string
		sourceEdited bool   // the source's body changes during the run
		bindEdit     string // written to the bound file during the run; "" leaves it
		expectedReal *string
	}{
		{
			caseName:     "SourceEdited",
			sourceEdited: true,
		},
		{
			caseName:     "SourceEditedAndBindEdited",
			sourceEdited: true,
			bindEdit:     "memory",
			expectedReal: new("memory"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			body := "shared"
			s := fileShare(func() ([]byte, error) { return []byte(body), nil })

			_, cleanup, err := s.Begin()
			require.NoError(t, err)

			if tc.bindEdit != "" {
				writeFile(t, bindFile(), tc.bindEdit)
			}
			if tc.sourceEdited {
				body = "host edit"
			}

			require.NoError(t, cleanup())

			if tc.expectedReal == nil {
				assert.True(t, lstatGone(t, realFile()), "the host's edit promotes nothing")
			} else {
				assert.Equal(t, *tc.expectedReal, readFile(t, realFile()))
				assertRegular(t, realFile())
			}
			assert.True(t, gone(t, bindFile()))
			assert.True(t, gone(t, baseFile()))
		})
	}
}

// TestShare_Cleanup_HostChangedDuringCrashedRun: the leftover clean diffs against the
// crashed run's baseline, so host edits made while down are not promoted
func TestShare_Cleanup_HostChangedDuringCrashedRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	body := "shared"
	s := fileShare(func() ([]byte, error) { return []byte(body), nil })

	// a crashed run leaves its scratch, baseline, and symlink
	crashRun(t, symlinkWiring)

	// the host edits while it's down
	body = "host edit"

	// the next run cleans up the leftovers: nothing promoted
	require.NoError(t, s.clean())
	assert.True(t, lstatGone(t, realFile()))
	assert.True(t, gone(t, bindFile()))
	assert.True(t, gone(t, baseFile()))

	// re-binds the host's fresh doc
	bind, cleanup, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, bindFile(), bind)
	assert.Equal(t, "host edit", readFile(t, bindFile()))
	assert.Equal(t, "host edit", readFile(t, baseFile()))
	require.NoError(t, cleanup())
}

// TestShare_Clean_Unchanged: an untouched dir scratch leaves nothing behind
func TestShare_Clean_Unchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := dirShare()

	_, clean, err := s.Begin()
	require.NoError(t, err)
	require.NoError(t, clean())

	assert.NoDirExists(t, dirScratch())
	assert.NoDirExists(t, dirReal(), "an untouched folder leaves nothing behind")
}

// TestShare_Clean_Changed: the run's tree promotes into Real
func TestShare_Clean_Changed(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		rel      string // written under the scratch during the run
		body     string // "" adds only an empty dir
	}{
		{
			caseName: "FileAdded",
			rel:      "notes/ideas.txt",
			body:     "keep this",
		},
		{
			caseName: "EmptyDirOnly", // an added empty dir promotes too
			rel:      "notes",
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			s := dirShare()

			_, clean, err := s.Begin()
			require.NoError(t, err)
			if tc.body == "" {
				require.NoError(t, os.MkdirAll(filepath.Join(dirScratch(), tc.rel), ioutil.Dir))
			} else {
				writeScratch(t, tc.rel, tc.body)
			}

			require.NoError(t, clean())

			assertSeeded(t, filepath.Join(dirReal(), "README.md"))
			if tc.body == "" {
				info, err := os.Stat(filepath.Join(dirReal(), tc.rel))
				require.NoError(t, err)
				assert.True(t, info.IsDir(), "an added empty dir promotes too")
			} else {
				assertFile(t, filepath.Join(dirReal(), tc.rel), tc.body)
			}
			assert.NoDirExists(t, dirScratch())
		})
	}
}

// TestShare_Clean_IntoExistingReal: the run's tree promotes into an existing Real,
// whose differing copies move aside as variants
func TestShare_Clean_IntoExistingReal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := dirShare()

	_, clean, err := s.Begin()
	require.NoError(t, err)

	// the run changes the seeded README, adds files; Real gains its own copies meanwhile
	writeScratch(t, "README.md", "run's edit")
	writeScratch(t, "notes/new.txt", "new file")
	writeScratch(t, "notes/same.txt", "same")
	require.NoError(t, os.MkdirAll(dirReal(), ioutil.Dir))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(dirReal(), "README.md"), []byte("real's edit")))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(dirReal(), "notes/keep.txt"), []byte("keep")))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(dirReal(), "notes/same.txt"), []byte("same")))

	require.NoError(t, clean())

	// the run's copy is promoted; Real's differing copy kept aside as a variant
	assertFile(t, filepath.Join(dirReal(), "README.md"), "run's edit")
	variants, err := filepath.Glob(filepath.Join(dirReal(), "README.run-*.md"))
	require.NoError(t, err)
	require.Len(t, variants, 1)
	assertFile(t, variants[0], "real's edit")

	// the run's new file lands in; the identical file leaves one copy; Real-only file untouched
	assertFile(t, filepath.Join(dirReal(), "notes/new.txt"), "new file")
	assertFile(t, filepath.Join(dirReal(), "notes/same.txt"), "same")
	sameVariants, err := filepath.Glob(filepath.Join(dirReal(), "notes/same.run-*"))
	require.NoError(t, err)
	assert.Empty(t, sameVariants, "an identical file promotes without a variant")
	assertFile(t, filepath.Join(dirReal(), "notes/keep.txt"), "keep")
	assert.NoDirExists(t, dirScratch())
}

// crashRun binds the wiring's share in a child process that exits without cleanup:
// the kernel releases the crashed run's registry entry, as a real crash does
func crashRun(t *testing.T, w wiring) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperCrashRun$") // #nosec G204,G702 -- re-execs the test binary itself
	cmd.Env = append(os.Environ(), "CCBOX_CRASH_RUN="+w.name)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", out)
}

// TestHelperCrashRun is crashRun's child process
func TestHelperCrashRun(t *testing.T) {
	name := os.Getenv("CCBOX_CRASH_RUN")
	if name == "" {
		t.Skip("helper: not under test")
	}
	for _, w := range wirings {
		if w.name == name {
			_, _, err := w.newShare().Begin()
			require.NoError(t, err)
			return
		}
	}
	t.Fatalf("helper: unknown wiring %s", name)
}
