package dock

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemuxReader_Read(t *testing.T) {
	big := strings.Repeat("x", 1200)

	var framed bytes.Buffer
	_, err := stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("one\ntwo\n"))
	require.NoError(t, err)
	_, err = stdcopy.NewStdWriter(&framed, stdcopy.Stderr).Write([]byte("three\n"))
	require.NoError(t, err)
	_, err = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte(big))
	require.NoError(t, err)

	// byte-wise reads force a frame's bytes to survive across reads
	got, err := io.ReadAll(iotest.OneByteReader(&demuxReader{r: io.NopCloser(&framed)}))
	require.NoError(t, err)
	assert.Equal(t, "one\ntwo\nthree\n"+big, string(got))
}
