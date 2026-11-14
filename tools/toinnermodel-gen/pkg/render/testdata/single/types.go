// Package single is toinnermodel-gen's golden fixture: run over this dir must
// equal the committed zz_generated.toinnermodel.single.go exactly.
package single

// full carries every supported shape pair
//
//toinnermodel:gen
type full struct {
	Plain      string
	Kept       *string
	Flag       *bool
	StructType *inner
}

type Full struct {
	Plain      string
	Kept       *string
	Flag       bool
	StructType Inner
	Computed   string // target-only: left zero
}

// inner is full's structType, discovered through full.StructType's shape pair
type inner struct {
	Name string
	Flag *bool
}

type Inner struct {
	Name  string
	Flag  bool
	Extra []string // target-only: left zero
}
