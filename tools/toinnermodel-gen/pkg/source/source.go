// Package source parses a Go package dir into toinnermodel-gen's struct sources:
// every struct by name and the //toinnermodel:gen-marked ones.
package source

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	markerPrefix = "//toinnermodel:" // the marker head

	markerGen     = "gen"     // marks struct to generate, nested structTypes need no mark
	markerSkip    = "skip"    // drop the field from pairing
	markerConvert = "convert" // call a func to convert to the innerModel field

	// GeneratedPrefix prefixes the tool's generated file, one per package
	GeneratedPrefix = "zz_generated.toinnermodel."
)

// Pkg is a package as toinnermodel-gen sees it: its name, the marked structs, and
// every struct and package-level func by name
type Pkg struct {
	Name          string
	MarkedStructs []Struct
	StructMap     map[string]Struct
	FuncMap       map[string]Func
}

// Scan parses dir's Go sources, skipping tests and the generated file.
func Scan(dir string) (Pkg, error) {
	p := Pkg{StructMap: map[string]Struct{}, FuncMap: map[string]Func{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Pkg{}, err
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if !isSource(entry) {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return Pkg{}, err
		}
		p.scanFile(file)
	}
	if p.Name == "" {
		return Pkg{}, fmt.Errorf("toinnermodel-gen: no Go sources in %s", dir)
	}
	return p, nil
}

// scanFile records the file's structs, keeping the annotated ones marked, and the
// package-level funcs the convert markers may name
func (p *Pkg) scanFile(file *ast.File) {
	p.Name = file.Name.Name
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			p.scanFunc(decl)
		case *ast.GenDecl:
			if decl.Tok != token.TYPE {
				continue
			}
			for _, spec := range decl.Specs {
				p.scanTypeSpec(decl, spec)
			}
		}
	}
}

// scanFunc records one package-level func; a method never answers to a marker
func (p *Pkg) scanFunc(fn *ast.FuncDecl) {
	if fn.Recv != nil {
		return
	}
	p.FuncMap[fn.Name.Name] = Func{In: fieldTypes(fn.Type.Params), Out: fieldTypes(fn.Type.Results)}
}

// fieldTypes lists the field list's types, one per name
func fieldTypes(fl *ast.FieldList) []string {
	if fl == nil {
		return nil
	}
	var list []string
	for _, astField := range fl.List {
		typ := types.ExprString(astField.Type)
		if len(astField.Names) == 0 {
			list = append(list, typ)
			continue
		}
		for range astField.Names {
			list = append(list, typ)
		}
	}
	return list
}

// scanTypeSpec records one struct spec as data; the marker sits on the doc group above it
func (p *Pkg) scanTypeSpec(gen *ast.GenDecl, spec ast.Spec) {
	typeSpec, ok := spec.(*ast.TypeSpec)
	if !ok {
		return
	}
	st, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return
	}
	strct := Struct{Name: typeSpec.Name.Name, Fields: fieldsOf(st)}
	p.StructMap[strct.Name] = strct
	if hasMarker(gen.Doc) || hasMarker(typeSpec.Doc) {
		p.MarkedStructs = append(p.MarkedStructs, strct)
	}
}

// fieldsOf flattens the struct's fields, one entry per name
func fieldsOf(st *ast.StructType) []Field {
	var fields []Field
	for _, astField := range st.Fields.List {
		typ := types.ExprString(astField.Type)
		if len(astField.Names) == 0 { // an embedded field is accessed by its type's name — a pointer's, sans *
			fields = append(fields, Field{Name: strings.TrimPrefix(typ, "*"), Type: typ})
			continue
		}
		skip, convertFn := fieldMarkers(astField.Doc, astField.Comment)
		for _, name := range astField.Names {
			fields = append(fields, Field{Name: name.Name, Type: typ, Skip: skip, ConvertFn: convertFn})
		}
	}
	return fields
}

// fieldMarkers reads the field's doc and line comments for the tool's markers
func fieldMarkers(doc, line *ast.CommentGroup) (bool, string) {
	var skip bool
	var convertFn string
	for _, group := range []*ast.CommentGroup{doc, line} {
		if group == nil {
			continue
		}
		for _, c := range group.List {
			directive, ok := strings.CutPrefix(strings.TrimSpace(c.Text), markerPrefix)
			if !ok {
				continue
			}
			switch name, arg, _ := strings.Cut(directive, " "); name {
			case markerSkip:
				skip = true
			case markerConvert:
				convertFn = strings.TrimSpace(arg)
			}
		}
	}
	return skip, convertFn
}

func hasMarker(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	return slices.ContainsFunc(doc.List, func(c *ast.Comment) bool {
		return strings.TrimSpace(c.Text) == markerPrefix+markerGen
	})
}

// isSource reports whether the dir entry is a hand-written Go source: tests and
// the generated files never contribute structs
func isSource(entry os.DirEntry) bool {
	name := entry.Name()
	return !entry.IsDir() && strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") && !strings.HasPrefix(name, GeneratedPrefix)
}

// Struct is a package struct as toinnermodel-gen sees it: its name and fields
type Struct struct {
	Name   string
	Fields []Field
}

// Field is one struct field: its name and type as written, and its markers' asks
type Field struct {
	Name string
	Type string // e.g. "string", "*vnc"
	// Skip drops the field from the pairing: the innerModel names no twin
	Skip bool
	// ConvertFn names the package func converting the field's type to the twin's
	ConvertFn string
}

// Func is a package-level func as toinnermodel-gen sees it: its arg and result types
type Func struct {
	In  []string
	Out []string
}
