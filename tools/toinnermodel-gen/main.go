// Command toinnermodel-gen writes a package's generated toInnerModel conversions for
// its //toinnermodel:gen structs — run via `go generate`, before any `go build`.
package main

import (
	"errors"
	"flag"
	"go/format"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/pair"
	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/render"
	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/source"
)

func main() {
	dir := flag.String("dir", ".", "package dir to scan for //toinnermodel:gen structs")
	flag.Parse()

	if err := run(*dir); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}

// run scans dir's sources, pairs every marked struct, and writes the
// package's one generated file
func run(dir string) error {
	pkg, err := source.Scan(dir)
	if err != nil {
		return err
	}

	walk := pair.NewWalk(pkg.StructMap)
	var pairs []pair.Pair
	var errs []error
	for _, marked := range pkg.MarkedStructs {
		p, err := walk.Pair(marked)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		pairs = append(pairs, p)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	body, err := format.Source(render.NewFile(pkg.Name, pairs).Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, source.GeneratedPrefix+pkg.Name+".go"), body, osutil.File) // #nosec G304 -- the caller's own output path
}
