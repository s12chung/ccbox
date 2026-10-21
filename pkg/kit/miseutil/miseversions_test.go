package miseutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubVersions serves the fixture lists at the versions service's data paths
func stubVersions(t *testing.T, lists map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, list := range lists {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(list))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	VersionsURL = server.URL
	t.Cleanup(func() { VersionsURL = VersionsBaseURL })
}

// versionListTmpl renders a versions-service TOML list — every release dated
var versionListTmpl = template.Must(template.New("versionList").Funcs(template.FuncMap{
	"rfc3339": func(at time.Time) string { return at.UTC().Format(time.RFC3339Nano) },
}).Parse(`[versions]
{{ range $version, $at := . }}{{ printf "%q" $version }} = { created_at = {{ rfc3339 $at }} }
{{ end }}`))

// versionList renders a versions-service TOML list dating every release
func versionList(versions map[string]time.Time) string {
	return renderTmpl(versionListTmpl, versions)
}

func TestToolURL(t *testing.T) {
	assert.Equal(t, VersionsBaseURL+"/data/playwright.toml", toolURL("npm:playwright"))
	assert.Equal(t, VersionsBaseURL+"/data/go.toml", toolURL("go"))
}

func TestMinorPin(t *testing.T) {
	assert.Equal(t, "1.27", minorPin("1.27.2"))
	assert.Equal(t, "26.12", minorPin("26.12.0"))
	assert.Equal(t, "0.11", minorPin("0.11.0"))
	assert.Equal(t, "3.13", minorPin("3.13"))
	assert.Equal(t, "4", minorPin("4"))
}

func TestVersionsSource_List(t *testing.T) {
	t.Run("a backend prefix drops off the tool's name", func(t *testing.T) {
		at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		stubVersions(t, map[string]string{"/data/playwright.toml": versionList(map[string]time.Time{"1.64.1": at})})

		entries, err := versionsSource{}.List(t.Context(), "npm:playwright")

		require.NoError(t, err)
		assert.Equal(t, []VersionEntry{{Version: "1.64.1", CreatedAt: &at}}, entries)
	})

	t.Run("an unlisted tool errors", func(t *testing.T) {
		stubVersions(t, nil)

		_, err := versionsSource{}.List(t.Context(), "bats")

		require.ErrorContains(t, err, "404 Not Found")
	})
}
