package tinyproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestListenAddr(t *testing.T) {
	// pkginfo's addr:port is the shared contract: the proxy env dials them too
	assert.Equal(t, pkginfo.ProxyAddr+":"+pkginfo.ProxyPort, listenAddr)
}
