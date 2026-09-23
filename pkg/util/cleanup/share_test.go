package cleanup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSharer records its lifecycle, settling without error
type fakeSharer struct {
	dir     string
	err     error // Begin's error
	begun   bool
	cleaned bool
}

func (f *fakeSharer) Begin() (string, func() error, error) {
	if f.err != nil {
		return "", nil, f.err // a failed Begin lays nothing to settle
	}
	f.begun = true
	return f.dir, func() error { f.cleaned = true; return nil }, nil
}

func TestShare_Begin(t *testing.T) {
	first, second := &fakeSharer{dir: "/a"}, &fakeSharer{dir: "/b"}

	var firstDir, secondDir string
	clean, err := Share{first, second}.Begin(&firstDir, &secondDir)
	require.NoError(t, err)
	require.NotNil(t, clean)
	assert.True(t, first.begun && second.begun, "each Sharer begun, in order")
	assert.Equal(t, "/a", firstDir)
	assert.Equal(t, "/b", secondDir)

	require.NoError(t, clean())
	assert.True(t, first.cleaned && second.cleaned, "clean settles every Sharer")
}

func TestShare_Begin_SharerError(t *testing.T) {
	wantErr := errors.New("boom")
	first, second := &fakeSharer{dir: "/a"}, &fakeSharer{err: wantErr}

	var firstDir, secondDir string
	clean, err := Share{first, second}.Begin(&firstDir, &secondDir)
	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, clean, "clean is returned on every path")
	assert.Equal(t, "/a", firstDir, "a Sharer begun before the failure still sets its dir")

	require.NoError(t, clean())
	assert.True(t, first.cleaned, "the begun Sharer still settles")
	assert.False(t, second.cleaned, "the failed Sharer laid nothing to settle")
}

func TestShare_Begin_SizeMismatch(t *testing.T) {
	var dir string
	tests := []struct {
		name string
		dirs []*string
		want string
	}{
		{name: "missing pointers", dirs: nil, want: "dir pointer count (0) does not match Sharer count (1)"},
		{name: "extra pointers", dirs: []*string{&dir, &dir}, want: "dir pointer count (2) does not match Sharer count (1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean, err := Share{&fakeSharer{dir: "/a"}}.Begin(tt.dirs...)
			require.ErrorContains(t, err, tt.want)
			assert.Nil(t, clean, "nothing begun, nothing to settle")
		})
	}
}

func TestShare_Begin_Empty(t *testing.T) {
	clean, err := Share{}.Begin()
	require.NoError(t, err)
	require.NotNil(t, clean)
	require.NoError(t, clean())
}
