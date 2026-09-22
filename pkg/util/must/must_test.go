package must

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	assert.Equal(t, 1, Get(1, nil))
	assert.PanicsWithError(t, "boom", func() { Get(1, errors.New("boom")) })
}

func TestDo(t *testing.T) {
	assert.NotPanics(t, func() { Do(nil) })
	assert.PanicsWithError(t, "boom", func() { Do(errors.New("boom")) })
}
