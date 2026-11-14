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
	// markerGen in a struct's doc group marks an input struct to generate for —
	// nested structTypes need no mark
	markerGen = "//toinnermodel:gen"

	// GeneratedPrefix prefixes the tool's generated file, one per package
	GeneratedPrefix = "zz_generated.toinnermodel."
)

// Pkg is a package as toinnermodel-gen sees it: its name, the marked structs, and
// every struct by name — pair resolves innerModels and structTypes against the map
type Pkg struct {
	Name          string
	MarkedStructs []Struct
	StructMap     map[string]Struct
}

// Scan parses dir's Go sources, skipping tests and the generated file.
func Scan(dir string) (Pkg, error) {
	p := Pkg{StructMap: map[string]Struct{}}
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

// scanFile records the file's structs, keeping the annotated ones marked
func (p *Pkg) scanFile(file *ast.File) {
	p.Name = file.Name.Name
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			p.scanTypeSpec(gen, spec)
		}
	}
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
		for _, name := range astField.Names {
			fields = append(fields, Field{Name: name.Name, Type: typ})
		}
	}
	return fields
}

func hasMarker(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	return slices.ContainsFunc(doc.List, func(c *ast.Comment) bool {
		return strings.TrimSpace(c.Text) == markerGen
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

// Field is one struct field: its name and type as written
type Field struct {
	Name string
	Type string // e.g. "string", "*vnc"
}
