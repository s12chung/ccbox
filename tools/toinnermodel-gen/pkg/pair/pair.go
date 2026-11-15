// Package pair pairs scanned structs against their innerModels: one Pair per
// struct, a StructType field nesting its own — the fields are the recursion.
package pair

import (
	"errors"
	"fmt"
	"strings"

	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/source"
)

// Walk pairs a package's structs against their innerModels, tracking the structs on
// the path — a repeat is a cycle
type Walk struct {
	structs map[string]strut
	funcs   map[string]source.Func
	path    map[string]bool
}

// NewWalk wraps every scanned struct for pairing, keyed the same
func NewWalk(pkg source.Pkg) Walk {
	w := Walk{structs: make(map[string]strut, len(pkg.StructMap)), funcs: pkg.FuncMap}
	for name, strct := range pkg.StructMap {
		w.structs[name] = strut{strct}
	}
	return w
}

// structTypeOf resolves a struct-typed shape's pairing — the same struct, modulo
// case, pointers carried across; anything looser is a guess
func (w Walk) structTypeOf(in, out string) (strut, strut, bool) {
	if strings.HasPrefix(out, "*") && !strings.HasPrefix(in, "*") {
		return strut{}, strut{}, false // a value input has no pointer to land on
	}
	name := strings.TrimPrefix(in, "*")
	outName := strings.TrimPrefix(out, "*")
	structType, ok := w.structs[name]
	if !ok {
		return strut{}, strut{}, false
	}
	structTypeInnerModel, ok := w.structs[outName]
	if !ok || name == outName || !strings.EqualFold(name, outName) {
		return strut{}, strut{}, false
	}
	return structType, structTypeInnerModel, true
}

// enter marks name as on the path, reporting false on a repeat
func (w Walk) enter(name string) bool {
	onPath := !w.path[name]
	w.path[name] = true
	return onPath
}

func (w Walk) exit(name string) { delete(w.path, name) }

// Pair pairs marked against its innerModel — one pairing per field, nesting each
// structType's own Pair. Dups pair again; the render lays each struct down once
func (w Walk) Pair(marked source.Struct) (Pair, error) {
	innerModelName := strings.ToUpper(marked.Name[:1]) + marked.Name[1:] // names marked's expected innerModel: config -> Config
	innerModel, ok := w.structs[innerModelName]
	if !ok {
		return Pair{}, fmt.Errorf("toinnermodel-gen: %s: no innerModel struct %s in the package", marked.Name, innerModelName)
	}
	w.path = map[string]bool{marked.Name: true}
	return strut{marked}.pair(innerModel, w)
}

// strut wraps a scanned struct for pairing
type strut struct{ source.Struct }

// field returns the struct's named field, reporting false when absent
func (marked strut) field(name string) (field, bool) {
	for _, f := range marked.Fields {
		if f.Name == name {
			return field{f}, true
		}
	}
	return field{}, false
}

// pair pairs marked against innerModel recursively
func (marked strut) pair(innerModel strut, w Walk) (Pair, error) {
	pair := Pair{Marked: marked.Struct, InnerModel: innerModel.Struct}
	var errs []error
	for _, f := range marked.Fields {
		if f.Skip {
			continue // no twin: a retired field parses, never converts
		}
		twinField, ok := innerModel.field(f.Name)
		if !ok {
			errs = append(errs, fmt.Errorf("toinnermodel-gen: %s.%s: no field %s on innerModel %s", marked.Name, f.Name, f.Name, innerModel.Name))
			continue
		}
		fieldPair, err := field{f}.pair(twinField, w)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pair.Fields = append(pair.Fields, fieldPair)
	}
	return pair, errors.Join(errs...)
}

// field wraps a scanned field for pairing
type field struct{ source.Field }

// pair pairs field with twinField, choosing the setting from the shape pair
func (markedField field) pair(twinField field, w Walk) (FieldPair, error) {
	in, out := markedField.Type, twinField.Type
	if markedField.ConvertFn != "" {
		return markedField.convert(in, out, w)
	}
	switch {
	case in == out:
		return Assign{Field: markedField.Field}, nil
	case in == "*"+out && !strings.HasPrefix(out, "*"): // multi-level pointers have no conversion
		return Deref{Field: markedField.Field}, nil
	}
	structType, structTypeInnerModel, ok := w.structTypeOf(in, out)
	if !ok {
		return nil, fmt.Errorf("toinnermodel-gen: %s: unsupported shape %s -> %s", markedField.Name, in, out)
	}
	// a structType already on the path is a cycle — pairing it again recurses
	// forever
	if !w.enter(structType.Name) {
		return StructType{Field: markedField.Field, TwinType: twinField.Type}, nil
	}
	defer w.exit(structType.Name)
	structTypePair, err := structType.pair(structTypeInnerModel, w)
	if err != nil {
		return nil, err
	}
	return StructType{Field: markedField.Field, TwinType: twinField.Type, Pair: &structTypePair}, nil
}

// convert checks the marker-named package func takes the input type and returns the twin's
func (markedField field) convert(in, out string, w Walk) (FieldPair, error) {
	fn, ok := w.funcs[markedField.ConvertFn]
	if !ok {
		return nil, fmt.Errorf("toinnermodel-gen: %s: no func %s in the package", markedField.Name, markedField.ConvertFn)
	}
	if len(fn.In) != 1 || len(fn.Out) != 1 || fn.In[0] != in || fn.Out[0] != out {
		return nil, fmt.Errorf("toinnermodel-gen: %s: %s is func(%s) (%s), want func(%s) (%s)",
			markedField.Name, markedField.ConvertFn, strings.Join(fn.In, ", "), strings.Join(fn.Out, ", "), in, out)
	}
	return Convert{Field: markedField.Field, Fn: markedField.ConvertFn}, nil
}

// Pair is one marked struct's conversion: its innerModel and one FieldPair per field
type Pair struct {
	Marked     source.Struct
	InnerModel source.Struct
	Fields     []FieldPair
}

// FieldPair is one field's pairing — one type per setting
type FieldPair interface{ isFieldPair() }

// Assign moves a same-shape field across
type Assign struct{ Field source.Field }

func (Assign) isFieldPair() {}

// Deref derefs an input pointer onto its value target, nil-guarded
type Deref struct{ Field source.Field }

func (Deref) isFieldPair() {}

// Convert converts an input field via the marker-named package func
type Convert struct {
	Field source.Field
	Fn    string
}

func (Convert) isFieldPair() {}

// StructType converts an input struct — pointer or value — to the target via
// its toInnerModel, nil-guarded on a pointer.
type StructType struct {
	Field    source.Field
	TwinType string // twin field's type — the shape the conversion lands on
	Pair     *Pair  // nil on cycle
}

func (StructType) isFieldPair() {}
