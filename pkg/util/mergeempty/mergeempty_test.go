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

	src := &struct{ Value string }{"old"}
	cp := Ptr(src, nil)
	cp.Value = "new"
	assert.Equal(t, "old", src.Value) // a struct pointee is deep-copied, not aliased
}

func TestStruct(t *testing.T) {
	type nested struct{ V string }
	type section struct {
		S string
		P *string
		N *nested
		L []string
		M map[string]int
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

	// lists set on both sides append, deep-copied — not aliased
	al, bl := []string{"a"}, []string{"b"}
	out = Struct(&section{L: al}, &section{L: bl})
	require.NotNil(t, out.L)
	assert.Equal(t, []string{"a", "b"}, out.L)
	out.L[0] = "x"
	out.L[1] = "y"
	assert.Equal(t, []string{"a"}, al)
	assert.Equal(t, []string{"b"}, bl)

	// maps set on both sides merge per key, b winning — deep-copied too
	am := map[string]int{"k": 1}
	out = Struct(&section{M: am}, &section{M: map[string]int{"k": 2, "j": 3}})
	require.NotNil(t, out.M)
	assert.Equal(t, map[string]int{"k": 2, "j": 3}, out.M)
	out.M["k"] = 9
	assert.Equal(t, map[string]int{"k": 1}, am)
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

func TestMerge(t *testing.T) {
	type nested struct{ V string }
	type section struct {
		S string
		K string
		P *string
		Q *string
		N *nested
		L []string
		M map[string]int
	}

	keep, win := "keep", "win"
	dst := &section{
		S: keep,
		K: keep,
		P: &keep,
		Q: &keep,
		N: &nested{V: keep},
		L: []string{"a"},
		M: map[string]int{"k": 1},
	}
	src := section{
		S: win,
		P: &win,
		N: &nested{}, // the nested struct's zero field keeps dst's
		L: []string{"b"},
		M: map[string]int{"j": 2, "k": 3},
	}

	Merge(dst, src)

	assert.Equal(t, &section{
		S: win,                            // a set src scalar wins
		K: keep,                           // a field unset in src inherits
		P: &win,                           // a set src pointer wins
		Q: &keep,                          // a pointer unset in src keeps dst's
		N: &nested{V: keep},               // struct pointers set on both sides merge per field
		L: []string{"a", "b"},             // lists append
		M: map[string]int{"j": 2, "k": 3}, // maps merge per key, src winning
	}, dst)
}

func TestMerge_DoesNotAliasSrc(t *testing.T) {
	type nested struct{ V string }
	type section struct {
		P *string
		N *nested
		L []string
		M map[string]int
	}

	v := "v"
	src := &section{P: &v, N: &nested{V: "v"}, L: []string{"a"}, M: map[string]int{"k": 1}}
	dst := &section{}

	Merge(dst, *src)

	*dst.P = "x"
	dst.N.V = "x"
	dst.L[0] = "x"
	dst.M["k"] = 2

	assert.Equal(t, "v", *src.P) // every write deep-copies src
	assert.Equal(t, &nested{V: "v"}, src.N)
	assert.Equal(t, []string{"a"}, src.L)
	assert.Equal(t, map[string]int{"k": 1}, src.M)
}

func TestMerge_KeepsDstUnexported(t *testing.T) {
	type section struct {
		S     string
		hides string
	}

	dst := &section{S: "a", hides: "dst"}
	src := section{S: "b", hides: "src"}

	Merge(dst, src)

	assert.Equal(t, "b", dst.S)
	assert.Equal(t, "dst", dst.hides, "dst's unexported fields stay its own — reflection can't write them")
}

func TestMerge_PreservesNonNilEmpty(t *testing.T) {
	type section struct {
		L []string
		M map[string]int
	}

	dst := &section{L: []string{}, M: map[string]int{}}
	Merge(dst, section{})
	assert.NotNil(t, dst.L) // a non-nil empty survives an unset src, not collapsed
	assert.NotNil(t, dst.M)

	dst = &section{}
	Merge(dst, section{L: []string{}})
	assert.NotNil(t, dst.L) // src's non-nil empty lands even on a nil dst
	assert.Nil(t, dst.M)
}
