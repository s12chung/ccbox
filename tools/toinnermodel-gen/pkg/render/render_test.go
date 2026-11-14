package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/pair"
	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/source"
)

func TestFile(t *testing.T) {
	file := NewFile("single", []pair.Pair{pairOf("full",
		assignField("Plain"),
		structTypeField("StructType", pairOf("inner", derefField("Flag"))),
	)})
	assert.Equal(t, fileHeader+"\n\npackage single\n\n"+
		"func (x *full) toInnerModel() Full {\n\tvar out Full\n"+
		"\tout.Plain = x.Plain\n"+
		"\tif x.StructType != nil {\n\t\tout.StructType = x.StructType.toInnerModel()\n\t}\n"+
		"\treturn out\n}\n\n"+
		"func (x *inner) toInnerModel() Inner {\n\tvar out Inner\n"+
		"\tif x.Flag != nil {\n\t\tout.Flag = *x.Flag\n\t}\n"+
		"\treturn out\n}\n\n", string(file.Bytes()))
}

func TestFile_Claims(t *testing.T) {
	tests := []struct {
		name  string
		pairs []pair.Pair
		funcs string
	}{
		{"a shared structType renders once, in the first referencing pair's place", []pair.Pair{
			pairOf("alpha", structTypeField("Shared", pairOf("shared", assignField("Term")))),
			pairOf("beta", structTypeField("Shared", pairOf("shared", assignField("Term")))),
		}, "alpha\nshared\nbeta"},
		{"a marked struct reached as a structType keeps its own func", []pair.Pair{
			pairOf("gamma", structTypeField("Alpha", pairOf("alpha", assignField("Term")))),
			pairOf("alpha", assignField("Term")),
		}, "gamma\nalpha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := NewFile("multi", tt.pairs)
			var names []string
			for _, fn := range file.funcs {
				names = append(names, fn.input)
			}
			assert.Equal(t, tt.funcs, strings.Join(names, "\n"))
		})
	}
}

func TestFunc_Statements(t *testing.T) {
	tests := []struct {
		name  string
		pair  pair.FieldPair
		lines string
	}{
		{"assign moves a same-shape field across", pair.Assign{Field: fld("Plain", "string")}, "\tout.Plain = x.Plain\n"},
		{
			"deref nil-guards an input pointer onto its value",
			pair.Deref{Field: fld("Flag", "*bool")},
			"\tif x.Flag != nil {\n\t\tout.Flag = *x.Flag\n\t}\n",
		},
		{
			"structType nil-guards a pointer struct's toInnerModel call",
			pair.StructType{Field: fld("StructType", "*inner")},
			"\tif x.StructType != nil {\n\t\tout.StructType = x.StructType.toInnerModel()\n\t}\n",
		},
		{
			"structType assigns a value struct's toInnerModel directly",
			pair.StructType{Field: fld("Inner", "inner")},
			"\tout.Inner = x.Inner.toInnerModel()\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := NewFunc(pair.Pair{
				Marked:     source.Struct{Name: "full"},
				InnerModel: source.Struct{Name: "Full"},
				Fields:     []pair.FieldPair{tt.pair},
			})
			assert.Equal(t, "func (x *full) toInnerModel() Full {\n\tvar out Full\n"+tt.lines+"\treturn out\n}\n\n", string(fn.Bytes()))
		})
	}
}

func TestNewStatement_UnknownSetting(t *testing.T) {
	assert.Panics(t, func() { newStatement(unknownPairing{}) })
}

// unknownPairing stands in for a FieldPair setting the render doesn't know —
// the embed satisfies the interface; the type switch doesn't match it
type unknownPairing struct{ pair.Assign }

// pairOf lays input's conversion out against its capitalized innerModel, with fields
func pairOf(input string, fields ...pair.FieldPair) pair.Pair {
	return pair.Pair{
		Marked:     source.Struct{Name: input},
		InnerModel: source.Struct{Name: strings.ToUpper(input[:1]) + input[1:]},
		Fields:     fields,
	}
}

func assignField(name string) pair.FieldPair {
	return pair.Assign{Field: fld(name, "string")}
}

func derefField(name string) pair.FieldPair {
	return pair.Deref{Field: fld(name, "*bool")}
}

func fld(name, typ string) source.Field {
	return source.Field{Name: name, Type: typ}
}

// structTypeField lays a structType's call out — nil Pair models a cycle's back-reference
func structTypeField(name string, structType ...pair.Pair) pair.FieldPair {
	fieldPair := pair.StructType{Field: fld(name, "*inner")}
	if len(structType) > 0 {
		fieldPair.Pair = &structType[0]
	}
	return fieldPair
}
