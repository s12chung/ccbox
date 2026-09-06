package pkger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNpmArg(t *testing.T) {
	assert.Equal(t, "npm:@scope/pkg", Npm{Package: "@scope/pkg"}.Arg())
}

func TestNpmLatest(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version": "1.2.3"}`))
		}))
		defer srv.Close()

		v, err := latestAt(srv.URL, "@scope/pkg")
		require.NoError(t, err)
		assert.Equal(t, "1.2.3", v)
	})

	for name, body := range map[string]string{
		"http error": `not found`,
		"no version": `{}`,
		"bad json":   `<html></html>`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			_, err := latestAt(srv.URL, "@scope/pkg")
			assert.Error(t, err)
		})
	}
}

func TestNpmValidate(t *testing.T) {
	tests := []struct {
		name string
		pkg  string
		want []string
	}{
		{"scoped", "@anthropic-ai/claude-code", nil},
		{"plain", "opencode-ai", nil},
		{"empty", "", []string{"Package.Match"}},
		{"uppercase", "MyCLI", []string{"Package.Match"}},
		{"unscoped slash", "foo/bar", []string{"Package.Match"}},
		{"scope without name", "@scope", []string{"Package.Match"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(Npm{Package: tt.pkg})
			if tt.want == nil {
				assert.Nil(t, errMap.ToNil())
				return
			}
			require.NotEmpty(t, errMap)
			for _, want := range tt.want {
				assert.Contains(t, errMap.Error(), want)
			}
		})
	}
}
