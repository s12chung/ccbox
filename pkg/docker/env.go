package docker

import (
	"sort"
	"strconv"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// envString renders env as KEY=VALUE in sorted-key order, the proxy vars last — overriding
// user-supplied settings; the container reads their presence as the egress wall being on
func envString(env map[string]string, noProxy bool) []string {
	renderedEnv := sortedEnv(env)
	if noProxy {
		return renderedEnv
	}
	return append(renderedEnv, []string{
		// the target is the fixed IP: no name resolves inside the child netns, whose
		// loopback isn't the parent's
		"http_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		"https_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
		// Loopback never leaves the container, so route it direct — else local dev servers
		// and browsers hit the proxy and get refused.
		"no_proxy=localhost,127.0.0.1,::1",
		"NO_PROXY=localhost,127.0.0.1,::1",
	}...)
}

// sortedEnv renders a KEY=VALUE list in sorted-key order for a deterministic spec.
func sortedEnv(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+m[k])
	}
	return env
}
