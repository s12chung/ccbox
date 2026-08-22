package mergeempty

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	assert.Nil(t, Slice[string](nil, nil))              // both nil → nil (drives allowlist defaults)
	assert.Equal(t, []string{}, Slice([]string{}, nil)) // non-nil empty preserved, not collapsed
	assert.Equal(t, []string{"a", "b", "c"}, Slice([]string{"a"}, []string{"b", "c"}))

	a := []string{"a"}
	out := Slice(a, []string{"b"})
	out[0] = "x"
	assert.Equal(t, []string{"a"}, a) // fresh: shares no backing storage with either input
}

func TestMap(t *testing.T) {
	assert.Nil(t, Map[string, int](nil, nil))                     // both nil → nil
	assert.Equal(t, map[string]int{}, Map(map[string]int{}, nil)) // non-nil empty preserved
	assert.Equal(t, map[string]int{"a": 1, "b": 3, "c": 3},
		Map(map[string]int{"a": 1, "b": 2}, map[string]int{"b": 3, "c": 3})) // b wins on collision

	a := map[string]int{"a": 1}
	out := Map(a, map[string]int{"b": 2})
	out["a"] = 9
	assert.Equal(t, map[string]int{"a": 1}, a) // fresh: mutating result doesn't reach input
}
