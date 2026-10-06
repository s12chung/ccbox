package log

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefer(t *testing.T) {
	var buf bytes.Buffer
	old := outErr
	outErr = &buf
	defer func() { outErr = old }()

	// nil error → nothing logged.
	Defer("close thing", func() error { return nil })
	assert.Zero(t, buf.Len(), "nil error should log nothing")

	// real error → a line naming the action and the error.
	Defer("close thing", func() error { return errors.New("boom") })
	got := buf.String()
	for _, want := range []string{"close thing failed", "boom"} {
		assert.Contains(t, got, want)
	}
}
