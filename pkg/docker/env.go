package docker

import (
	"strconv"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// envString renders env as KEY=VALUE in sorted-key order, the proxy vars last — overriding
// user-supplied settings; the container reads their presence as the egress wall being on
func envString(env map[string]string, isProxied, hasHostGateway bool) []string {
	renderedEnv := dock.SortedEnv(env)
	if !isProxied {
		return renderedEnv
	}
	noProxy := "localhost,127.0.0.1,::1"
	if hasHostGateway {
		// the host gateway's flows bypass tinyproxy — its http must too, else the
		// allowlist refuses it
		noProxy += "," + pkginfo.HostGatewayHost
	}
	return append(renderedEnv, []string{
		// the target is the fixed IP: no name resolves inside the child netns, whose
		// loopback isn't the parent's
		"http_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		"https_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		// Loopback never leaves the container, so route it direct — else local dev servers
		// and browsers hit the proxy and get refused.
		"no_proxy=" + noProxy,
		"NO_PROXY=" + noProxy,
	}...)
}
