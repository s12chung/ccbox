package deepcopy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type sample struct {
	List []string
	M    map[string]int
	P    *int
}

func TestOfIsIndependent(t *testing.T) {
	n := 1
	src := sample{List: []string{"a"}, M: map[string]int{"k": 1}, P: &n}
	cp := Of(src)

	// mutating every reference-typed field of the copy must not reach src
	cp.List[0] = "b"
	cp.M["k"] = 2
	cp.M["new"] = 3
	*cp.P = 9

	assert.Equal(t, []string{"a"}, src.List)
	assert.Equal(t, map[string]int{"k": 1}, src.M)
	assert.Equal(t, 1, n)
}

func TestOfPreservesNil(t *testing.T) {
	cp := Of(sample{}) // nil slice/map/pointer stay nil, not empty
	assert.Nil(t, cp.List)
	assert.Nil(t, cp.M)
	assert.Nil(t, cp.P)
}
