package hubkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHub serves the token service and the registry off one fixture server
func stubHub(t *testing.T, tags []string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"token":"stub-token"}`))
		case "/v2/library/debian/tags/list":
			if r.Header.Get("Authorization") != "Bearer stub-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			body, err := json.Marshal(map[string]any{"name": "debian", "tags": tags})
			assert.NoError(t, err)
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	origToken, origList := tokenURLFormat, listURLFormat
	tokenURLFormat = srv.URL + "/token?service=registry.docker.io&scope=repository:%s:pull"
	listURLFormat = srv.URL + "/v2/%s/tags/list"
	t.Cleanup(func() { tokenURLFormat, listURLFormat = origToken, origList })
	return srv.URL
}

// trixieSlimRe matches the dated snapshot slim tags — never the floating alias
var trixieSlimRe = regexp.MustCompile(`^trixie-\d{8}-slim$`)

func TestLatestTag(t *testing.T) {
	stubHub(t, []string{"trixie-slim", "bookworm-20250113-slim", "trixie-20250113-slim", "trixie-20250929-slim"})

	tag, err := LatestTag(t.Context(), "library/debian", trixieSlimRe)

	require.NoError(t, err)
	assert.Equal(t, "trixie-20250929-slim", tag, "the matches sorting lexically over the whole listing")
}

func TestLatestTag_Errors(t *testing.T) {
	t.Run("a listing with nothing matching errors, carrying the repo", func(t *testing.T) {
		stubHub(t, []string{"trixie-slim"})

		_, err := LatestTag(t.Context(), "library/debian", trixieSlimRe)

		require.ErrorContains(t, err, "hub: library/debian: the listing has no tag matching")
	})

	t.Run("a failed fetch errors", func(t *testing.T) {
		base := stubHub(t, nil)
		orig := listURLFormat
		listURLFormat = base + "/v2/%s/missing"
		t.Cleanup(func() { listURLFormat = orig })

		_, err := LatestTag(t.Context(), "library/debian", trixieSlimRe)

		require.ErrorContains(t, err, "404 Not Found")
	})
}
