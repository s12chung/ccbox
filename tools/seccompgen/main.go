// moby/profiles/seccomp is linux-only, so this generates only on linux
//go:build ignore

// Command seccompgen regenerates seccomp_default.json from the library's live
// DefaultProfile — a linux-box task: the library gates the call to linux.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/moby/profiles/seccomp"

	"github.com/s12chung/ccbox/pkg/util/osutil"
)

func main() {
	out := flag.String("o", "", "output path for the rendered default profile")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/seccompgen/main.go -o <output path>")
		os.Exit(2)
	}
	rendered, err := json.MarshalIndent(seccomp.DefaultProfile(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, append(rendered, '\n'), osutil.File); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
