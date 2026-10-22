// Command termlint lints the repo for banned terms configured in
// .termlint.json (make lint.terms). make runs it via `go run`: it lints
// source text, not the built binary, so it must work before `go build`.
package main

import (
	"os"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/termlint/pkg/termlint"
)

func main() {
	if err := termlint.Run("."); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}
