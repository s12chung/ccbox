package maputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	m := map[string]int{"a": 1}

	v, ok := Get(m, "a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = Get(m, "missing")
	assert.False(t, ok)
	assert.Zero(t, v)

	s, ok := Get[int, string](nil, 1)
	assert.False(t, ok)
	assert.Empty(t, s)
}

func TestNilIfEmpty(t *testing.T) {
	assert.Nil(t, NilIfEmpty(map[string]int{}))
	assert.Nil(t, NilIfEmpty(map[string]int(nil)))
	assert.Equal(t, map[string]int{"a": 1}, NilIfEmpty(map[string]int{"a": 1}))
}
