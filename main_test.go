package main

import (
	"bufio"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBuildContextHasCopySources guards the embed/Dockerfile coupling: every path
// the Dockerfile COPYs from the build context must be present in buildContext.
// Fails if the embed globs drift from the Dockerfile.
func TestBuildContextHasCopySources(t *testing.T) {
	f, err := buildContext.Open("Dockerfile")
	require.NoError(t, err, "Dockerfile not embedded")
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || fields[0] != "COPY" {
			continue
		}

		fromStage := false
		var nonFlag []string
		for _, a := range fields[1:] {
			switch {
			case strings.HasPrefix(a, "--from="):
				fromStage = true
			case !strings.HasPrefix(a, "--"):
				nonFlag = append(nonFlag, a)
			}
		}
		// --from copies from a build stage, not the context; the last arg is the dest.
		if fromStage || len(nonFlag) < 2 {
			continue
		}
		for _, src := range nonFlag[:len(nonFlag)-1] {
			src = strings.TrimSuffix(src, "/") // dir COPYs carry a trailing slash
			_, err := fs.Stat(buildContext, src)
			require.NoErrorf(t, err, "Dockerfile COPYs %q but it isn't embedded in buildContext", src)
		}
	}
	require.NoError(t, sc.Err())
}
