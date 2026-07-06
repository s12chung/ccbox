package docker

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowOverrideRendersRegex(t *testing.T) {
	got := AllowOverride([]string{"example.com", "test.mywebsite.com"})

	body, ok := got[allowFileName]
	require.True(t, ok)
	assert.Equal(t, "(^|\\.)example\\.com$\n(^|\\.)test\\.mywebsite\\.com$\n", string(body))
}

// A rendered entry matches the apex and any subdomain, but not a lookalike sharing the suffix.
func TestRenderAllowMatchesSubdomains(t *testing.T) {
	// tinyproxy applies the filter file line-by-line, so match one rendered line.
	re := regexp.MustCompile(strings.TrimSpace(string(renderAllow([]string{"cloudfront.net"}))))

	for _, host := range []string{"cloudfront.net", "d123.cloudfront.net", "a.b.cloudfront.net"} {
		assert.True(t, re.MatchString(host), host)
	}
	for _, host := range []string{"evilcloudfront.net", "cloudfront.net.evil.com"} {
		assert.False(t, re.MatchString(host), host)
	}
}
