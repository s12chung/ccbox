package proxy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
		want    string
	}{
		{"Domain", []string{"example.com"}, "(^|\\.)example\\.com$\n"},
		{"Subdomain", []string{"a.b.example.com"}, "(^|\\.)a\\.b\\.example\\.com$\n"},
		{"Domains", []string{"example.com", "registry.npmjs.org"}, "(^|\\.)example\\.com$\n(^|\\.)registry\\.npmjs\\.org$\n"},
		{"EmptyAllowlist", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(Render(tt.domains)))
		})
	}
}

// A rendered rule matches the apex and any subdomain, but not a lookalike sharing the suffix.
func TestRender_MatchesSubdomains(t *testing.T) {
	// tinyproxy applies the filter file line-by-line, so match one rendered rule.
	re := regexp.MustCompile(strings.TrimSpace(string(Render([]string{"cloudfront.net"}))))

	for _, host := range []string{"cloudfront.net", "d123.cloudfront.net", "a.b.cloudfront.net"} {
		assert.True(t, re.MatchString(host), host)
	}
	for _, host := range []string{"evilcloudfront.net", "cloudfront.net.evil.com"} {
		assert.False(t, re.MatchString(host), host)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"Domains", "(^|\\.)example\\.com$\n(^|\\.)a\\.b\\.example\\.com$\n", []string{"example.com", "a.b.example.com"}},
		{"NoTrailingNewline", "(^|\\.)example\\.com$", []string{"example.com"}},
		{"BlankLines", "\n(^|\\.)example\\.com$\n\n", []string{"example.com"}},
		{"EmptyBody", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.body)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParse_Malformed(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"BareDomain", "example.com\n"},
		{"MissingAnchor", "(^|\\.)example\\.com"},
		{"WrongPrefix", "(.|\\.)example\\.com$\n"},
		{"NonDotEscape", "(^|\\.)bad\\w\\.com$\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "malformed")
		})
	}
}

func TestDomains(t *testing.T) {
	t.Run("MissingFile", func(t *testing.T) {
		_, err := Domains(filepath.Join(t.TempDir(), "allow.txt"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no proxy")
	})

	// the bound allow file is the proxy's own render: parse gives the config's domains back
	t.Run("RenderRoundtrip", func(t *testing.T) {
		domains := []string{"example.com", "registry.npmjs.org", "a.b.example.com"}

		path := filepath.Join(t.TempDir(), "allow.txt")
		require.NoError(t, os.WriteFile(path, Render(domains), 0o600))

		got, err := Domains(path)
		require.NoError(t, err)
		assert.Equal(t, domains, got)
	})
}
