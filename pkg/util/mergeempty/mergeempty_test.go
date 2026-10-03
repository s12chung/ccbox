package mergeempty

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPtr(t *testing.T) {
	assert.Nil(t, Ptr[string](nil, nil)) // both nil → nil

	a, b := "a", "b"
	out := Ptr(&a, &b)
	require.NotNil(t, out)
	assert.Equal(t, "b", *out)
	*out = "x"
	assert.Equal(t, "b", b) // fresh: shares no storage with either input

	out = Ptr(&a, nil)
	require.NotNil(t, out)
	assert.Equal(t, "a", *out) // nil b keeps a
	*out = "y"
	assert.Equal(t, "a", a)

	src := &struct{ Resolution string }{"1600x900"}
	cp := Ptr(src, nil)
	cp.Resolution = "31x31"
	assert.Equal(t, "1600x900", src.Resolution) // a struct pointee is deep-copied, not aliased
}

func TestStruct(t *testing.T) {
	type nested struct{ V string }
	type section struct {
		S string
		P *string
		N *nested
		L []string
	}

	assert.Nil(t, Struct[section](nil, nil)) // both nil → nil

	// b wins per set field: S unset in b keeps a's
	out := Struct(&section{S: "a", N: &nested{V: "a"}}, &section{S: "b"})
	assert.Equal(t, &section{S: "b", N: &nested{V: "a"}}, out)

	// a struct pointer overlays recursively — nested zero fields keep a's too
	out = Struct(&section{N: &nested{V: "a"}}, &section{N: &nested{}})
	assert.Equal(t, &section{N: &nested{V: "a"}}, out)

	// fresh: mutating the result doesn't reach either input
	a, b := &section{S: "a", N: &nested{V: "a"}}, &section{N: &nested{V: "b"}}
	out = Struct(a, b)
	out.S = "x"
	out.N.V = "x"
	assert.Equal(t, &section{S: "a", N: &nested{V: "a"}}, a)
	assert.Equal(t, &section{N: &nested{V: "b"}}, b)

	// b's nil nested keeps a's
	out = Struct(&section{N: &nested{V: "a"}}, &section{})
	assert.Equal(t, &section{N: &nested{V: "a"}}, out)

	// a's nil nested takes b's, deep-copied — not aliased
	bN := &nested{V: "b"}
	out = Struct(&section{}, &section{N: bN})
	require.NotNil(t, out.N)
	assert.NotSame(t, bN, out.N)
	out.N.V = "x"
	assert.Equal(t, "b", bN.V)

	// a value pointer set on both sides is deep-copied too — not aliased
	av, bv := "a", "b"
	out = Struct(&section{P: &av}, &section{P: &bv})
	require.NotNil(t, out.P)
	assert.Equal(t, "b", *out.P)
	*out.P = "x"
	assert.Equal(t, "b", bv)

	// a slice set on both sides is deep-copied too — not aliased
	al, bl := []string{"a"}, []string{"b"}
	out = Struct(&section{L: al}, &section{L: bl})
	require.NotNil(t, out.L)
	assert.Equal(t, "b", out.L[0])
	out.L[0] = "x"
	assert.Equal(t, []string{"a"}, al)
	assert.Equal(t, []string{"b"}, bl)
}

func TestStruct_UnexportedFieldsZeroed(t *testing.T) {
	type section struct {
		S     string
		hides string
	}

	out := Struct(&section{S: "a", hides: "x"}, &section{S: "b"})

	require.NotNil(t, out)
	assert.Equal(t, "b", out.S)
	assert.Empty(t, out.hides, "unexported fields carry from neither side")
}
