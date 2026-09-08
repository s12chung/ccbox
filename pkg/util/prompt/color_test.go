package prompt

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColor_Wrap(t *testing.T) {
	assert.Equal(t, "\x1b[31mhi\x1b[0m", ColorRed.Wrap("hi"))
	assert.Equal(t, "hi", Color("").Wrap("hi"), "empty color is a no-op")
}

// redLines tints every line red; "" lines (here, blank ones) stay uncolored.
func redLines(line string) Color {
	if len(line) <= 1 {
		return ""
	}
	return ColorRed
}

// A bytes.Buffer isn't a terminal, so NewColorWriter returns it unwrapped; the
// coloring tests below construct ColorWriter directly to exercise that path.
func TestNewColorWriter_PassesNonTTYThrough(t *testing.T) {
	var buf bytes.Buffer
	assert.Equal(t, io.Writer(&buf), NewColorWriter(&buf, redLines))
}

func TestColorWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	cw := &ColorWriter{w: &buf, colorer: redLines}

	_, err := cw.Write([]byte("hi\n\n"))
	require.NoError(t, err)
	assert.Equal(t, "\x1b[31mhi\x1b[0m\n\n", buf.String(),
		"known line colored, uncolored line passes through byte-for-byte")
}

func TestColorWriter_WriteBuffersSplitLine(t *testing.T) {
	var buf bytes.Buffer
	cw := &ColorWriter{w: &buf, colorer: redLines}

	_, err := cw.Write([]byte("hel"))
	require.NoError(t, err)
	assert.Empty(t, buf.String(), "no newline yet, nothing emitted")

	_, err = cw.Write([]byte("lo\n"))
	require.NoError(t, err)
	assert.Equal(t, "\x1b[31mhello\x1b[0m\n", buf.String(),
		"split line emitted once, fully colored")
}
