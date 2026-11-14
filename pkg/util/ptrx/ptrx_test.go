package ptrx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeref(t *testing.T) {
	assert.Empty(t, Deref[string](nil))
	assert.Nil(t, Deref[[]string](nil))

	p := "a"
	assert.Equal(t, "a", Deref(&p))
}
