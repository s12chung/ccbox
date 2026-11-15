package pair

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/source"
)

func TestPair(t *testing.T) {
	structs := map[string]source.Struct{
		"full": {Name: "full", Fields: []source.Field{
			{Name: "Plain", Type: "string"},
			{Name: "Kept", Type: "*string"},
			{Name: "Flag", Type: "*bool"},
			{Name: "StructType", Type: "*inner"},
			{Name: "Filter", Type: "*filter"},
			{Name: "Inner", Type: "inner"},
			{Name: "Outer", Type: "*outer"},
			{Name: "Ports", Type: "string", ConvertFn: "splitPorts"},
			{Name: "Retired", Type: "*bool", Skip: true},
		}},
		"Full": {Name: "Full", Fields: []source.Field{
			{Name: "Plain", Type: "string"},
			{Name: "Kept", Type: "*string"},
			{Name: "Flag", Type: "bool"},
			{Name: "StructType", Type: "Inner"},
			{Name: "Filter", Type: "Filter"},
			{Name: "Inner", Type: "Inner"},
			{Name: "Outer", Type: "*Outer"},
			{Name: "Ports", Type: "[]string"},
		}},
		"inner":  {Name: "inner", Fields: []source.Field{{Name: "Flag", Type: "*bool"}}},
		"Inner":  {Name: "Inner", Fields: []source.Field{{Name: "Flag", Type: "bool"}}},
		"filter": {Name: "filter", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"Filter": {Name: "Filter", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"outer":  {Name: "outer", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"Outer":  {Name: "Outer", Fields: []source.Field{{Name: "Term", Type: "string"}}},
	}
	funcs := map[string]source.Func{
		"splitPorts": {In: []string{"string"}, Out: []string{"[]string"}},
	}
	w := NewWalk(source.Pkg{StructMap: structs, FuncMap: funcs})
	pair, err := w.Pair(structs["full"])
	require.NoError(t, err)

	assert.Equal(t, Pair{
		Marked:     structs["full"],
		InnerModel: structs["Full"],
		Fields: []FieldPair{
			Assign{Field: fld("Plain", "string")},
			Assign{Field: fld("Kept", "*string")},
			Deref{Field: fld("Flag", "*bool")},
			StructType{Field: fld("StructType", "*inner"), TwinType: "Inner", Pair: &Pair{
				Marked:     structs["inner"],
				InnerModel: structs["Inner"],
				Fields:     []FieldPair{Deref{Field: fld("Flag", "*bool")}},
			}},
			StructType{Field: fld("Filter", "*filter"), TwinType: "Filter", Pair: &Pair{
				Marked:     structs["filter"],
				InnerModel: structs["Filter"],
				Fields:     []FieldPair{Assign{Field: fld("Term", "string")}},
			}},
			StructType{Field: fld("Inner", "inner"), TwinType: "Inner", Pair: &Pair{
				Marked:     structs["inner"],
				InnerModel: structs["Inner"],
				Fields:     []FieldPair{Deref{Field: fld("Flag", "*bool")}},
			}},
			StructType{Field: fld("Outer", "*outer"), TwinType: "*Outer", Pair: &Pair{
				Marked:     structs["outer"],
				InnerModel: structs["Outer"],
				Fields:     []FieldPair{Assign{Field: fld("Term", "string")}},
			}},
			Convert{Field: structs["full"].Fields[7], Fn: "splitPorts"},
			// Retired is skipped — renders nothing
		},
	}, pair)
}

func TestPair_StructTypes(t *testing.T) {
	structs := map[string]source.Struct{
		"alpha":  {Name: "alpha", Fields: []source.Field{{Name: "Shared", Type: "*shared"}, {Name: "Alias", Type: "*shared"}, {Name: "Own", Type: "*own"}}},
		"Alpha":  {Name: "Alpha", Fields: []source.Field{{Name: "Shared", Type: "Shared"}, {Name: "Alias", Type: "Shared"}, {Name: "Own", Type: "Own"}}},
		"beta":   {Name: "beta", Fields: []source.Field{{Name: "Shared", Type: "*shared"}}},
		"Beta":   {Name: "Beta", Fields: []source.Field{{Name: "Shared", Type: "Shared"}}},
		"gamma":  {Name: "gamma", Fields: []source.Field{{Name: "Alpha", Type: "*alpha"}}},
		"Gamma":  {Name: "Gamma", Fields: []source.Field{{Name: "Alpha", Type: "Alpha"}}},
		"shared": {Name: "shared", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"Shared": {Name: "Shared", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"own":    {Name: "own", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"Own":    {Name: "Own", Fields: []source.Field{{Name: "Term", Type: "string"}}},
	}
	w := NewWalk(source.Pkg{StructMap: structs})
	pair, err := w.Pair(structs["alpha"])
	require.NoError(t, err)

	// every structType nests its own Pair, dups included — dedup is the render's
	// claim, not the pairing's
	require.Len(t, pair.Fields, 3)
	for i, want := range []string{"shared", "shared", "own"} {
		st := asStructType(t, pair.Fields[i])
		require.NotNil(t, st.Pair)
		assert.Equal(t, want, st.Pair.Marked.Name)
	}

	// gamma's alpha nests fully too — the mark only picks run's top-level
	// structs; the pairing ignores it
	gammaPair, err := w.Pair(structs["gamma"])
	require.NoError(t, err)
	require.Len(t, gammaPair.Fields, 1)
	gamma := asStructType(t, gammaPair.Fields[0])
	require.NotNil(t, gamma.Pair)
	assert.Equal(t, "alpha", gamma.Pair.Marked.Name)
}

func TestPair_Cycles(t *testing.T) {
	// a and b reference each other — pairing a terminates, nesting b once; the
	// back-reference's Pair is nil
	structs := map[string]source.Struct{
		"a": {Name: "a", Fields: []source.Field{{Name: "B", Type: "*b"}}},
		"A": {Name: "A", Fields: []source.Field{{Name: "B", Type: "B"}}},
		"b": {Name: "b", Fields: []source.Field{{Name: "A", Type: "*a"}}},
		"B": {Name: "B", Fields: []source.Field{{Name: "A", Type: "A"}}},
	}

	pair, err := NewWalk(source.Pkg{StructMap: structs}).Pair(structs["a"])
	require.NoError(t, err)
	require.Len(t, pair.Fields, 1)
	b := asStructType(t, pair.Fields[0])
	require.NotNil(t, b.Pair)
	require.Len(t, b.Pair.Fields, 1)
	a := asStructType(t, b.Pair.Fields[0])
	assert.Nil(t, a.Pair)
}

func TestPair_Refusals(t *testing.T) {
	structs := map[string]source.Struct{
		"widget":    {Name: "widget", Fields: []source.Field{{Name: "Name", Type: "string"}}},
		"inputonly": {Name: "inputonly", Fields: []source.Field{{Name: "Extra", Type: "string"}}},
		"Inputonly": {Name: "Inputonly"},
		"shapes": {Name: "shapes", Fields: []source.Field{
			{Name: "Double", Type: "**string"},
			{Name: "Value", Type: "string"},
			{Name: "Ptr", Type: "inner"},
		}},
		"Shapes": {Name: "Shapes", Fields: []source.Field{
			{Name: "Double", Type: "*string"},
			{Name: "Value", Type: "*string"},
			{Name: "Ptr", Type: "*inner"},
		}},
		"outer":  {Name: "outer", Fields: []source.Field{{Name: "Orphan", Type: "*orphan"}}},
		"Outer":  {Name: "Outer", Fields: []source.Field{{Name: "Orphan", Type: "Orphan"}}},
		"orphan": {Name: "orphan", Fields: []source.Field{{Name: "Term", Type: "string"}}},
		"nested": {Name: "nested", Fields: []source.Field{{Name: "Inner", Type: "*inner"}}},
		"Nested": {Name: "Nested", Fields: []source.Field{{Name: "Inner", Type: "Inner"}}},
		"inner":  {Name: "inner", Fields: []source.Field{{Name: "Extra", Type: "string"}}},
		"Inner":  {Name: "Inner"},
		"converter": {Name: "converter", Fields: []source.Field{
			{Name: "Missing", Type: "string", ConvertFn: "missing"},
			{Name: "Shifted", Type: "string", ConvertFn: "splitPorts"},
		}},
		"Converter": {Name: "Converter", Fields: []source.Field{
			{Name: "Missing", Type: "[]string"},
			{Name: "Shifted", Type: "int"},
		}},
	}
	funcs := map[string]source.Func{
		"splitPorts": {In: []string{"string"}, Out: []string{"[]string"}},
	}
	w := NewWalk(source.Pkg{StructMap: structs, FuncMap: funcs})
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			"a struct with no innerModel refuses",
			"widget",
			[]string{"no innerModel struct Widget in the package"},
		},
		{
			"an input-only field refuses",
			"inputonly",
			[]string{"no field Extra on innerModel Inputonly"},
		},
		{
			"unsupported shapes refuse",
			"shapes",
			[]string{"unsupported shape **string -> *string", "unsupported shape string -> *string", "unsupported shape inner -> *inner"},
		},
		{
			"a structType with no innerModel struct refuses",
			"outer",
			[]string{"unsupported shape *orphan -> Orphan"},
		},
		{
			"a nested structType's refusal bubbles up",
			"nested",
			[]string{"no field Extra on innerModel Inner"},
		},
		{
			"a convert func missing or mismatched refuses",
			"converter",
			[]string{"no func missing in the package", "splitPorts is func(string) ([]string), want func(string) (int)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.Pair(structs[tt.input])
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

// asStructType asserts fieldPair's setting is a StructType, returning it
func asStructType(t *testing.T, fieldPair FieldPair) StructType {
	t.Helper()
	st, ok := fieldPair.(StructType)
	require.True(t, ok, "expected StructType, got %T", fieldPair)
	return st
}

func fld(name, typ string) source.Field {
	return source.Field{Name: name, Type: typ}
}
