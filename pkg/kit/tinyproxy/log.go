package tinyproxy

import (
	"errors"
	"io"
	"net"
	"strings"

	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/pkg/util/term"
)

// Log streams tinyproxy's logs through Writer; Colored tints each line per level, and
// Stop closes when the stream ends — the foreground proxy exits on its own
type Log struct {
	Writer  io.WriteCloser
	Colored bool
	Stop    chan struct{}
}

// Close closes the Log's writer
func (l Log) Close() error {
	if l.Writer == nil {
		return nil
	}
	return l.Writer.Close()
}

// Stream streams logs through Writer
func (l Log) Stream(logs io.ReadCloser) error {
	var w io.Writer = l.Writer
	if l.Colored {
		w = term.NewColorWriter(w, levelColor)
	}
	_, err := stdcopy.StdCopy(w, w, logs)
	if l.Stop != nil {
		close(l.Stop)
	}
	return err
}

// StreamGo streams logs via Stream in a goroutine and returns a channel yielding
// its error when the stream ends.
func (l Log) StreamGo(logs io.ReadCloser) chan error {
	logDone := make(chan error)
	go func() {
		err := l.Stream(logs)
		if errors.Is(err, net.ErrClosed) { // cleanup closed the stream; not a real failure
			err = nil
		}
		logDone <- err
		close(logDone)
	}()
	return logDone
}

var levelColors = map[string]term.Color{
	"CRITICAL": term.ColorBoldRed,
	"ERROR":    term.ColorRed,
	"WARNING":  term.ColorYellow,
	"NOTICE":   term.ColorGreen,
	"CONNECT":  term.ColorCyan,
	"INFO":     term.ColorDim,
}

func levelColor(line string) term.Color {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return levelColors[fields[0]]
}
