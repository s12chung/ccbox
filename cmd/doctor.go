package cmd

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/util/tarutil"
)

var doctorCmd = &cobra.Command{
	Use:    "doctor",
	Short:  "Check build-time invariants of this ccbox binary",
	Hidden: true, // a maintainer command; runs at build time (see the Makefile)
}

var doctorToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Check the embedded ccboxtools tar matches the on-disk ccboxtools/ tree",
	RunE: func(_ *cobra.Command, _ []string) error {
		return validateToolsTar(buildContext, os.DirFS("."))
	},
}

func init() { doctorCmd.AddCommand(doctorToolsCmd) }

// validateToolsTar compares the context's packed ccboxtools tar.gz against the
// on-disk ccboxtools tree byte-for-byte
func validateToolsTar(contextFS, srcFS fs.FS) error {
	want, err := readTree(srcFS)
	if err != nil {
		return err
	}

	packed, err := readToolsTar(contextFS)
	if err != nil {
		return err
	}
	problems := diffProblems(packed, want)
	for _, problem := range problems {
		log.Errorf("%s", problem)
	}
	if len(problems) > 0 {
		return errors.New("stale dist/ccboxtools.tar.gz — run make to regenerate")
	}
	return nil
}

func readToolsTar(contextFS fs.FS) (map[string][]byte, error) {
	f, err := contextFS.Open("dist/ccboxtools.tar.gz")
	if err != nil {
		return nil, err
	}
	defer log.Defer("packed tar close", f.Close)
	return tarutil.ReadTar(f)
}

// readTree reads the on-disk ccboxtools tree into name → file body
func readTree(srcFS fs.FS) (map[string][]byte, error) {
	tree := map[string][]byte{}
	err := fs.WalkDir(srcFS, "ccboxtools", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		tree[path], err = fs.ReadFile(srcFS, path)
		return err
	})
	return tree, err
}

// diffProblems compares packed against want byte-for-byte, consuming want
func diffProblems(packed, want map[string][]byte) []string {
	var problems []string
	for _, name := range slices.Sorted(maps.Keys(packed)) {
		body, ok := want[name]
		switch {
		case !ok:
			problems = append(problems, name+" in the tar is not on disk")
		case !bytes.Equal(body, packed[name]):
			problems = append(problems, name+" drifted from the tar")
		}
		delete(want, name)
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		problems = append(problems, name+" is on disk but missing from the tar")
	}
	return problems
}
