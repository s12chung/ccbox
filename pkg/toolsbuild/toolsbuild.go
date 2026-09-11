// Package toolsbuild compiles the ccboxtools nested module with the canonical
// build —  shared by make (toolsbuild/) and `ccbox doctor tools`.
package toolsbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Build compiles the ccboxtools main package into relOutPath for linux/<goarch>.
// It must run from the repo root (the project dir).
func Build(ctx context.Context, goarch, relOutPath string) error {
	abs, err := filepath.Abs(relOutPath)
	if err != nil {
		return err
	}
	out, err := buildCmd(ctx, goarch, abs).CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build ccboxtools: %w\n%s", err, out)
	}
	return nil
}

func buildCmd(ctx context.Context, goarch, outPath string) *exec.Cmd {
	// #nosec G204 -- goarch/outPath come from make/doctor, not user input
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-o", outPath, ".")
	cmd.Dir = "ccboxtools" // relative to the project dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+goarch)
	return cmd
}
