package logmux

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemux(t *testing.T) {
	body, ok := Demux("tinyproxy", "tinyproxy: NOTICE   Jun 22 21:45:16.558 [1]: Reloading")
	assert.True(t, ok)
	assert.Equal(t, "NOTICE   Jun 22 21:45:16.558 [1]: Reloading", body)

	_, ok = Demux("tinyproxy", "boot: installing")
	assert.False(t, ok, "the boot's own lines mux under no name")

	_, ok = Demux("tinyproxy", "tinyproxy:")
	assert.False(t, ok, "the bare name is no line's tag")
}

func TestMuxer_TagsLines(t *testing.T) {
	var out bytes.Buffer
	m := NewMuxer("tinyproxy", &out)

	n, err := m.Write([]byte("one\ntw"))
	require.NoError(t, err)
	assert.Equal(t, len("one\ntw"), n, "whole writes report through the buffering")

	_, err = m.Write([]byte("o\nthree\n"))
	require.NoError(t, err)

	assert.Equal(t, "tinyproxy: one\ntinyproxy: two\ntinyproxy: three\n", out.String())
}

func TestDemuxer_FilesByKind(t *testing.T) {
	var boot, proxy bytes.Buffer
	d := &Demuxer{Writers: map[string]io.Writer{"": &boot, "tinyproxy": &proxy}}

	n, err := d.Write([]byte("boot: one\ntinyproxy: NOTICE two\npar"))
	require.NoError(t, err)
	assert.Equal(t, len("boot: one\ntinyproxy: NOTICE two\npar"), n, "whole writes report through the filing")

	_, err = d.Write([]byte("tial boot line\ntinyproxy: INFO three\n"))
	require.NoError(t, err)

	assert.Equal(t, "boot: one\npartial boot line\n", boot.String())
	assert.Equal(t, "NOTICE two\nINFO three\n", proxy.String(), "muxed lines file without their tag")
}

func TestDemuxer_Flush(t *testing.T) {
	var boot, proxy bytes.Buffer
	d := &Demuxer{Writers: map[string]io.Writer{"": &boot, "tinyproxy": &proxy}}

	_, err := d.Write([]byte("done\ntorn boot"))
	require.NoError(t, err)
	require.NoError(t, d.Flush())

	assert.Equal(t, "done\ntorn boot", boot.String())
	assert.Empty(t, proxy.String())

	_, err = d.Write([]byte("\ntinyproxy: torn proxy"))
	require.NoError(t, err)
	require.NoError(t, d.Flush())

	assert.Equal(t, "done\ntorn boot\n", boot.String())
	assert.Equal(t, "torn proxy", proxy.String(), "a torn line files by its kind, tag stripped")

	require.NoError(t, d.Flush(), "a drained demuxer flushes nothing")
}
