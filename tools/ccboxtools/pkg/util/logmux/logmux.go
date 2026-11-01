// Package logmux muxes and demuxes the boot's shared log stream: each writer's
// lines carry its tag, and readers split them apart by it.
package logmux

import (
	"bytes"
	"io"
	"strings"
)

// Muxer tags each complete line written through it with its Name — a muxed
// stream's container-side writer. Bytes buffer until a newline completes a line.
type Muxer struct {
	Name string // the tag ahead of a line's body, the demuxer's key

	w   io.Writer
	buf []byte
}

// NewMuxer returns a Muxer tagging lines into w under name.
func NewMuxer(name string, w io.Writer) *Muxer { return &Muxer{Name: name, w: w} }

func (m *Muxer) Write(p []byte) (int, error) {
	m.buf = append(m.buf, p...)
	for {
		i := bytes.IndexByte(m.buf, '\n')
		if i < 0 {
			break
		}
		if _, err := m.w.Write(m.tagLine(m.buf[:i+1])); err != nil {
			return len(p), err
		}
		m.buf = m.buf[i+1:]
	}
	return len(p), nil
}

func (m *Muxer) tagLine(line []byte) []byte {
	body := bytes.TrimRight(line, "\n")
	return []byte(tagPrefix(m.Name) + string(body) + "\n")
}

// Demux strips a muxed line's tag for name, yielding the body — false when the
// line doesn't mux under name.
func Demux(name, line string) (string, bool) {
	return strings.CutPrefix(line, tagPrefix(name))
}

// Demuxer files a muxed stream's lines by tag into Writers — each name's lines
// stripped of their tag, the boot's own lines whole under the empty name.
// Lines buffer until their newline lands, so every writer sees whole lines.
type Demuxer struct {
	Writers map[string]io.Writer
	buf     []byte
}

func (d *Demuxer) Write(p []byte) (int, error) {
	d.buf = append(d.buf, p...)
	for {
		i := bytes.IndexByte(d.buf, '\n')
		if i < 0 {
			break
		}
		if err := d.file(d.buf[:i+1]); err != nil {
			return len(p), err
		}
		d.buf = d.buf[i+1:]
	}
	return len(p), nil
}

// Flush files the stream's final partial line — without it, the line is lost.
func (d *Demuxer) Flush() error {
	if len(d.buf) == 0 {
		return nil
	}
	tail := d.buf
	d.buf = nil
	return d.file(tail)
}

// file writes a whole line, trailing newline kept, to the writer of the name
// it muxes under.
func (d *Demuxer) file(line []byte) error {
	name, body := "", line
	for n := range d.Writers {
		if n == "" {
			continue
		}
		if b, ok := bytes.CutPrefix(line, []byte(tagPrefix(n))); ok {
			name, body = n, b
			break
		}
	}
	_, err := d.Writers[name].Write(body)
	return err
}

func tagPrefix(name string) string { return name + ": " }
