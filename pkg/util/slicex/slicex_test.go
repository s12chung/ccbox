package slicex

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

func TestUnique(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, Unique([]int{1, 2, 1, 3, 2}))
	assert.Equal(t, []int{1}, Unique([]int{1, 1}))
	assert.Nil(t, Unique[int](nil))
}

func TestIntersect(t *testing.T) {
	assert.Equal(t, []string{"b"}, Intersect([]string{"a", "b", "c"}, []string{"b", "d"}))
	assert.Equal(t, []string{"c", "a"}, Intersect([]string{"c", "a"}, []string{"a", "c"}), "a's order wins")
	assert.Nil(t, Intersect([]string{"a"}, []string{"b"}))
	assert.Nil(t, Intersect[int](nil, []int{1}))
}

func TestMap(t *testing.T) {
	assert.Equal(t, []string{"1", "2"}, Map([]int{1, 2}, strconv.Itoa))
	assert.Empty(t, Map([]int(nil), strconv.Itoa))
}
