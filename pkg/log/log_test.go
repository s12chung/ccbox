package log

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefer(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	// nil error → nothing logged.
	Defer("close thing", func() error { return nil })
	assert.Zero(t, buf.Len(), "nil error should log nothing")

	// real error → a WARN line naming the action and the error.
	Defer("close thing", func() error { return errors.New("boom") })
	out := buf.String()
	for _, want := range []string{"WARN", "close thing failed", "boom"} {
		assert.Contains(t, out, want)
	}
}
