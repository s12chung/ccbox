// Package printutil prints sections as comment lines: a header line, then its
// items bulleted under it.
package printutil

import (
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Section is one printed group: a header line, then its items bulleted under
// it. Depth indents a section under the section printed above it.
type Section struct {
	Header string
	Items  []string
	Depth  int
}

// PrintSections prints the sections' comment lines in order
func PrintSections(sections []Section) {
	for _, line := range Render(sections) {
		log.Info(line)
	}
}

// Print prints the section's comment lines
func (s Section) Print() {
	for _, line := range s.lines() {
		log.Info(line)
	}
}

// Render renders the sections' lines in print order, blank-lining the
// top-level ones apart; nested sections hug the section printed above them
func Render(sections []Section) []string {
	var lines []string
	for i, s := range sections {
		if i > 0 && s.Depth == 0 {
			lines = append(lines, "")
		}
		lines = append(lines, s.lines()...)
	}
	return lines
}

// lines renders the section's lines: its header, then each item bulleted at
// its depth
func (s Section) lines() []string {
	pad := strings.Repeat("  ", s.Depth)
	lines := make([]string, 0, 1+len(s.Items))
	lines = append(lines, "# "+pad+s.Header)
	for _, item := range s.Items {
		lines = append(lines, "# "+pad+"  - "+item)
	}
	return lines
}
