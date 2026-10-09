package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	toolsbuild "github.com/s12chung/ccbox/tools/toolsbuild/pkg/build"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check this ccbox install",
}

var doctorToolsCmd = &cobra.Command{
	Use:    "tools",
	Short:  "Check the embedded ccboxtools binary is a fresh build of tools/ccboxtools/",
	Hidden: true, // a maintainer command; runs at build time (see the Makefile)
	RunE: func(cmd *cobra.Command, _ []string) error {
		embedded, err := fs.ReadFile(embedBuildContext, "dist/ccboxtools")
		if err != nil {
			return err
		}
		goarch, err := toolsGoarch(cmd.Context())
		if err != nil {
			return err
		}
		return validateTools(embedded, goarch, func(goarch string) ([]byte, error) {
			return rebuildTools(cmd.Context(), goarch)
		})
	},
}

var doctorClisCmd = &cobra.Command{
	Use:   "clis",
	Short: "Check the user-defined clis load",
	RunE: func(_ *cobra.Command, _ []string) error {
		return checkUserTree(clitmpl.UserDir(), "cli", cli.LoadUserClis)
	},
}

var doctorProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "Check the user-defined providers load",
	RunE: func(_ *cobra.Command, _ []string) error {
		return checkUserTree(provider.UserDir(), "provider", provider.LoadUserProviders)
	},
}

func init() { doctorCmd.AddCommand(doctorToolsCmd, doctorClisCmd, doctorProvidersCmd) }

// checkUserTree reports a user tree's load warnings as errors — bad entries are
// skipped with a startup warning, so doctor is where they surface.
func checkUserTree[T any](dir, noun string, load func() ([]T, []error, error)) error {
	_, warns, err := load()
	if err != nil {
		return err
	}
	for _, warn := range warns {
		log.Errorf("%s", warn)
	}
	if len(warns) > 0 {
		return fmt.Errorf("%d user %s(s) failed to load in %s", len(warns), noun, userdir.Tilde(dir))
	}
	return nil
}

// toolsGoarch is the image arch the embedded binary was built for: make passes
// GOARCH on the doctor recipe line; a bare run falls back to the compiler default,
// matching the Makefile's own GOARCH default
func toolsGoarch(ctx context.Context) (string, error) {
	if goarch := os.Getenv("GOARCH"); goarch != "" {
		return goarch, nil
	}
	out, err := exec.CommandContext(ctx, "go", "env", "GOARCH").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// validateTools byte-compares the embedded binary against a fresh canonical
// build (tools/build) — the compiler is the differ, so any drift in the
// tree, the toolchain or the flags is caught by construction
func validateTools(embedded []byte, goarch string, rebuild func(goarch string) ([]byte, error)) error {
	fresh, err := rebuild(goarch)
	if err != nil {
		return err
	}
	if !bytes.Equal(embedded, fresh) {
		log.Errorf("embedded dist/ccboxtools doesn't match a fresh linux/%s build of tools/ccboxtools/", goarch)
		return errors.New("stale dist/ccboxtools — run make")
	}
	return nil
}

func rebuildTools(ctx context.Context, goarch string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "ccbox-doctor") // one file, so just write at root
	if err != nil {
		return nil, err
	}
	defer log.Defer("doctor temp dir remove", func() error { return os.RemoveAll(dir) })

	out := filepath.Join(dir, "ccboxtools")
	if err := toolsbuild.Build(ctx, goarch, out); err != nil {
		return nil, err
	}
	return os.ReadFile(out) // #nosec G304 -- the temp dir's own fresh build
}
