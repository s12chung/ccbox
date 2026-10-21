package mise

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResolvedTool(t *testing.T) {
	tests := []struct {
		name  string
		tool  Tool
		state ToolState
		want  ResolvedTool
	}{
		{
			"the written pin rides, the postinstall as written",
			Tool{Name: "npm:playwright", PostInstall: "playwright install-deps chromium && playwright install chromium"},
			ToolState{Pins: map[string]string{"npm:playwright": "1.64"}},
			ResolvedTool{
				Name: "npm:playwright", Pin: "1.64",
				PostInstall: "playwright install-deps chromium && playwright install chromium",
			},
		},
		{
			"the unwritten pin fills off the version func",
			Tool{Name: "go"},
			ToolState{VersionFunc: pinned(map[string]string{"go": "1.27"})},
			ResolvedTool{Name: "go", Pin: "1.27"},
		},
		{
			"no postinstall — the plain pin, the comment riding",
			Tool{Name: "go", Comment: "# the toolchain"},
			ToolState{Pins: map[string]string{"go": "1.26"}},
			ResolvedTool{Name: "go", Pin: "1.26", Comment: "# the toolchain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := newResolvedTool(tt.tool, tt.state)

			require.NoError(t, err)
			assert.Equal(t, tt.want, rt)
		})
	}
}

func TestNewResolvedTool_Errors(t *testing.T) {
	t.Run("a fetch error propagates", func(t *testing.T) {
		_, err := newResolvedTool(Tool{Name: "go"}, ToolState{VersionFunc: func(string) (string, error) {
			return "", errors.New("the versions service is down")
		}})

		require.ErrorContains(t, err, "the versions service is down")
	})

	t.Run("a fetch coming back empty is the unresolved pin erroring", func(t *testing.T) {
		_, err := newResolvedTool(Tool{Name: "go"}, ToolState{
			VersionFunc: func(string) (string, error) { return "", nil },
		})

		require.ErrorContains(t, err, "mise: go: no pin resolved")
	})
}
