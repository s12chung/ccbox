package pypi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
)

// stubPyPI serves the fixture indexes at PyPI's JSON paths
func stubPyPI(t *testing.T, indexes map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, index := range indexes {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(index))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	JSONURL = server.URL
	t.Cleanup(func() { JSONURL = BaseURL })
}

func TestJSONURL(t *testing.T) {
	assert.Equal(t, BaseURL+"/pypi/setuptools/json", jsonURL("pypi:setuptools"))
}

func TestIndex_List(t *testing.T) {
	t.Run("reads the package's releases off its index", func(t *testing.T) {
		agedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		stubPyPI(t, map[string]string{
			"/pypi/wheel/json": `{"releases":{"0.48.0":[{"upload_time_iso_8601":"2026-01-01T00:00:00Z"}]}}`,
		})

		entries, err := Index{}.List(t.Context(), "pypi:wheel")

		require.NoError(t, err)
		assert.Equal(t, []miseutil.VersionEntry{{Version: "0.48.0", CreatedAt: &agedAt}}, entries)
	})

	t.Run("an unindexed package errors", func(t *testing.T) {
		stubPyPI(t, nil)

		_, err := Index{}.List(t.Context(), "pypi:bogus")

		require.ErrorContains(t, err, "404 Not Found")
	})
}

func TestIndex_Pin(t *testing.T) {
	assert.Equal(t, "82.0.1", Index{}.Pin("82.0.1"))
}

func TestIndexEntries(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ix := index{Releases: map[string][]file{
		"82.0.1": {{UploadTime: at.Format(time.RFC3339)}},
		"82.0.0": {},                      // its files all deleted
		"81.3.0": {{UploadTime: "bogus"}}, // an undatable file
	}}

	entries := ix.entries()

	require.Len(t, entries, 2, "an empty release — nothing to install — drops")
	byVersion := map[string]miseutil.VersionEntry{}
	for _, e := range entries {
		byVersion[e.Version] = e
	}
	assert.Equal(t, &at, byVersion["82.0.1"].CreatedAt, "dated by its first file")
	assert.Nil(t, byVersion["81.3.0"].CreatedAt, "an undatable file reads undated — aged")
}
