package uslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinus(t *testing.T) {
	assert.Equal(t, []string{"a", "c"}, Minus([]string{"a", "b", "c"}, []string{"b"}))
	assert.Nil(t, Minus([]string{"a"}, []string{"a"}))
	assert.Nil(t, Minus[int](nil, []int{1}))
}
