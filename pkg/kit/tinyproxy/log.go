package tinyproxy

import (
	"errors"
	"io"
	"net"
	"strings"

	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/pkg/util/prompt"
)

// Log streams tinyproxy's logs through Writer; Colored tints each line per level, and
// Stop closes when the stream ends — the foreground proxy exits on its own
type Log struct {
	Writer  io.Writer
	Colored bool
	Stop    chan struct{}
}

// Stream streams logs through Writer
func (l Log) Stream(logs io.ReadCloser) error {
	w := l.Writer
	if l.Colored {
		w = prompt.NewColorWriter(w, levelColor)
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

var levelColors = map[string]prompt.Color{
	"CRITICAL": prompt.ColorBoldRed,
	"ERROR":    prompt.ColorRed,
	"WARNING":  prompt.ColorYellow,
	"NOTICE":   prompt.ColorGreen,
	"CONNECT":  prompt.ColorCyan,
	"INFO":     prompt.ColorDim,
}

func levelColor(line string) prompt.Color {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return levelColors[fields[0]]
}
