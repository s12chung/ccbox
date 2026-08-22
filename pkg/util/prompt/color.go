package prompt

import (
	"bytes"
	"io"

	"github.com/moby/term"
)

// Color is an ANSI escape sequence tinting terminal output; ColorReset closes any of the others.
type Color string

// The colors ccbox uses.
const (
	ColorReset   Color = "\x1b[0m"
	ColorBoldRed Color = "\x1b[1;31m"
	ColorRed     Color = "\x1b[31m"
	ColorYellow  Color = "\x1b[33m"
	ColorGreen   Color = "\x1b[32m"
	ColorCyan    Color = "\x1b[36m"
	ColorDim     Color = "\x1b[2m"
)

// Wrap brackets s in c and a reset; the empty Color leaves s untouched.
func (c Color) Wrap(s string) string {
	if c == "" {
		return s
	}
	return string(c) + s + string(ColorReset)
}

// LineColorer picks the Color for a whole output line; "" leaves it uncolored.
type LineColorer func(line string) Color

// ColorWriter tints each line written through it via colorer. Upstream writers
// chunk arbitrarily, so bytes are buffered until a newline completes a line.
type ColorWriter struct {
	w       io.Writer
	colorer LineColorer
	buf     []byte
}

// NewColorWriter colors w when it's a terminal; otherwise it returns w
// untouched, so escapes never litter piped or redirected output.
func NewColorWriter(w io.Writer, colorer LineColorer) io.Writer {
	if _, isTerm := term.GetFdInfo(w); !isTerm {
		return w
	}
	return &ColorWriter{w: w, colorer: colorer}
}

func (cw *ColorWriter) Write(p []byte) (int, error) {
	cw.buf = append(cw.buf, p...)
	for {
		i := bytes.IndexByte(cw.buf, '\n')
		if i < 0 {
			break
		}
		if _, err := cw.w.Write(cw.colorLine(cw.buf[:i+1])); err != nil {
			return len(p), err
		}
		cw.buf = cw.buf[i+1:]
	}
	return len(p), nil
}

// colorLine tints line (its trailing newline kept outside the reset).
func (cw *ColorWriter) colorLine(line []byte) []byte {
	c := cw.colorer(string(line))
	if c == "" {
		return line
	}
	body := bytes.TrimRight(line, "\n")
	return []byte(c.Wrap(string(body)) + "\n")
}
