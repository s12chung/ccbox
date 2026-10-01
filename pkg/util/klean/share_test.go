package klean

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSharer records its lifecycle, cleaning without error
type fakeSharer struct {
	bind    ShareBind
	err     error // Begin's error
	begun   bool
	cleaned bool
}

func (f *fakeSharer) Begin() (ShareBind, func() error, error) {
	if f.err != nil {
		return ShareBind{}, nil, f.err // a failed Begin leaves nothing to clean
	}
	f.begun = true
	return f.bind, func() error { f.cleaned = true; return nil }, nil
}

func TestShare_Begin(t *testing.T) {
	first, second := &fakeSharer{bind: ShareBind{HostPath: "/a", ContainerPath: "/A"}},
		&fakeSharer{bind: ShareBind{HostPath: "/b"}}

	var firstBind, secondBind ShareBind
	clean, err := Share{first, second}.Begin(&firstBind, &secondBind)
	require.NoError(t, err)
	require.NotNil(t, clean)
	assert.True(t, first.begun && second.begun, "each Sharer begun, in order")
	assert.Equal(t, ShareBind{HostPath: "/a", ContainerPath: "/A"}, firstBind)
	assert.Equal(t, ShareBind{HostPath: "/b"}, secondBind)

	require.NoError(t, clean())
	assert.True(t, first.cleaned && second.cleaned, "every Sharer is cleaned")
}

func TestShare_Begin_SharerError(t *testing.T) {
	wantErr := errors.New("boom")
	first, second := &fakeSharer{bind: ShareBind{HostPath: "/a"}}, &fakeSharer{err: wantErr}

	var firstBind, secondBind ShareBind
	clean, err := Share{first, second}.Begin(&firstBind, &secondBind)
	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, clean, "clean is returned on every path")
	assert.Equal(t, ShareBind{HostPath: "/a"}, firstBind, "a Sharer begun before the failure still sets its bind")
	assert.Empty(t, secondBind, "the failed Sharer sets nothing")

	require.NoError(t, clean())
	assert.True(t, first.cleaned, "the begun Sharer is still cleaned")
	assert.False(t, second.cleaned, "the failed Sharer leaves nothing to clean")
}

func TestShare_Begin_SizeMismatch(t *testing.T) {
	var bind ShareBind
	tests := []struct {
		name  string
		binds []*ShareBind
		want  string
	}{
		{name: "missing pointers", binds: nil, want: "bind pointer count (0) does not match Sharer count (1)"},
		{name: "extra pointers", binds: []*ShareBind{&bind, &bind}, want: "bind pointer count (2) does not match Sharer count (1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean, err := Share{&fakeSharer{bind: ShareBind{HostPath: "/a"}}}.Begin(tt.binds...)
			require.ErrorContains(t, err, tt.want)
			assert.Nil(t, clean, "nothing begun, nothing to clean")
		})
	}
}

func TestShare_Begin_Empty(t *testing.T) {
	clean, err := Share{}.Begin()
	require.NoError(t, err)
	require.NotNil(t, clean)
	require.NoError(t, clean())
}
