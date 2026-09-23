package uslice

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinus(t *testing.T) {
	assert.Equal(t, []string{"a", "c"}, Minus([]string{"a", "b", "c"}, []string{"b"}))
	assert.Nil(t, Minus([]string{"a"}, []string{"a"}))
	assert.Nil(t, Minus[int](nil, []int{1}))
}

func TestMap(t *testing.T) {
	assert.Equal(t, []string{"1", "2"}, Map([]int{1, 2}, strconv.Itoa))
	assert.Empty(t, Map([]int(nil), strconv.Itoa))
}
