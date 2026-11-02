package docker

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestEnvString_BaseWins(t *testing.T) {
	got := envString(map[string]string{
		"GH_TOKEN":   "gh",
		"GOFLAGS":    "-mod=mod",
		"http_proxy": "evil",
	}, false)

	// o.Env sorted, then the proxy vars last — so the base http_proxy is the
	// final (winning) value.
	assert.Equal(t, []string{
		"GH_TOKEN=gh",
		"GOFLAGS=-mod=mod",
		"http_proxy=evil",
		"http_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		"https_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		"no_proxy=localhost,127.0.0.1,::1",
		"NO_PROXY=localhost,127.0.0.1,::1",
	}, got)
}

func TestEnvString_NoProxy(t *testing.T) {
	got := envString(map[string]string{"GH_TOKEN": "gh"}, true)
	// No proxy, no proxy vars — just the caller's env.
	assert.Equal(t, []string{"GH_TOKEN=gh"}, got)
}
