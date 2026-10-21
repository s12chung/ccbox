package mise

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pipCmdTmpl = `python -m pip install --no-cache-dir{{range .}} {{.Name}}=={{.Version}}{{end}}`

var pipNames = []string{"pypi:setuptools", "pypi:wheel"}

func TestNewResolvedTool(t *testing.T) {
	python := Tool{Name: "python", PostInstallCmdTmpl: pipCmdTmpl, PostInstallNames: pipNames}
	tests := []struct {
		name  string
		tool  Tool
		state ToolState
		want  ResolvedTool
	}{
		{
			"the written pins range into the cmd",
			python,
			ToolState{Pins: map[string]string{
				"python": "3.13", "pypi:setuptools": "82.0.1", "pypi:wheel": "0.47.0",
			}},
			ResolvedTool{
				Name: "python", Pin: "3.13",
				PostInstall: "python -m pip install --no-cache-dir setuptools==82.0.1 wheel==0.47.0",
			},
		},
		{
			"the unwritten pins fill off the version func",
			python,
			ToolState{
				Pins:        map[string]string{"python": "3.13"},
				VersionFunc: pinned(map[string]string{"pypi:setuptools": "84.0.0", "pypi:wheel": "0.48.0"}),
			},
			ResolvedTool{
				Name: "python", Pin: "3.13",
				PostInstall: "python -m pip install --no-cache-dir setuptools==84.0.0 wheel==0.48.0",
			},
		},
		{
			"the name renders sans its backend prefix",
			Tool{Name: "python", PostInstallCmdTmpl: "pip index versions {{range .}}{{.Name}}{{end}}", PostInstallNames: []string{"pypi:wheel"}},
			ToolState{VersionFunc: pinned(map[string]string{"python": "3.13", "pypi:wheel": "0.47.0"})},
			ResolvedTool{Name: "python", Pin: "3.13", PostInstall: "pip index versions wheel"},
		},
		{
			"a static cmd — no names — stays as written",
			Tool{Name: "npm:playwright", PostInstallCmdTmpl: "playwright install-deps chromium && playwright install chromium"},
			ToolState{Pins: map[string]string{"npm:playwright": "1.64"}},
			ResolvedTool{
				Name: "npm:playwright", Pin: "1.64",
				PostInstall: "playwright install-deps chromium && playwright install chromium",
			},
		},
		{
			"the config's written postinstall rides as-is — its packages never fetch",
			python,
			ToolState{
				Pins:         map[string]string{"python": "3.13", "pypi:setuptools": "82.0.1"}, // present yet bypassed
				PostInstalls: map[string]string{"python": "python -m pip install setuptools==70.0.0"},
			},
			ResolvedTool{Name: "python", Pin: "3.13", PostInstall: "python -m pip install setuptools==70.0.0"},
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
	python := Tool{Name: "python", PostInstallCmdTmpl: pipCmdTmpl, PostInstallNames: pipNames}

	t.Run("a fetch error propagates", func(t *testing.T) {
		_, err := newResolvedTool(python, ToolState{VersionFunc: func(string) (string, error) {
			return "", errors.New("the versions service is down")
		}})

		require.ErrorContains(t, err, "the versions service is down")
	})

	t.Run("a fetch coming back empty is the unresolved pin erroring", func(t *testing.T) {
		_, err := newResolvedTool(python, ToolState{
			Pins:        map[string]string{"python": "3.13"},
			VersionFunc: func(string) (string, error) { return "", nil },
		})

		require.ErrorContains(t, err, "mise: pypi:setuptools: no pin resolved")
	})
}
