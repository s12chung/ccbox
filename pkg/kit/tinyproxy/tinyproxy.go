// Package tinyproxy tints tinyproxy's log lines by level in the devbox's shared log stream.
package tinyproxy

import (
	"strings"

	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
)

var levelColors = map[string]term.Color{
	"CRITICAL": term.ColorBoldRed,
	"ERROR":    term.ColorRed,
	"WARNING":  term.ColorYellow,
	"NOTICE":   term.ColorGreen,
	"CONNECT":  term.ColorCyan,
	"INFO":     term.ColorDim,
}

// LevelColor tints tinyproxy's log lines by level; the boot's own lines stay
// uncolored.
func LevelColor(line string) term.Color {
	body, ok := logmux.Demux(pkginfo.ProxyMuxName, line)
	if !ok {
		return ""
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	return levelColors[fields[0]]
}
