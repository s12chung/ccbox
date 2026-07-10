package docker

import "sort"

// envString renders the container's environment as KEY=VALUE. The proxy/token vars come last so
// they win on a key collision (Docker takes the last value), keeping them unoverridable by o.Env.
func envString(o RunOptions) []string {
	base := []string{
		"http_proxy=http://" + egressName + ":" + proxyPort,
		"https_proxy=http://" + egressName + ":" + proxyPort,
		// Loopback never leaves the container, so route it direct — else local dev servers
		// and browsers hit the wall and get refused.
		"no_proxy=localhost,127.0.0.1,::1",
		"NO_PROXY=localhost,127.0.0.1,::1",
		"GH_TOKEN=" + o.GHToken,
	}

	keys := make([]string, 0, len(o.Env))
	for k := range o.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys) // stable order for a deterministic spec

	env := make([]string, 0, len(o.Env)+len(base))
	for _, k := range keys {
		env = append(env, k+"="+o.Env[k])
	}
	return append(env, base...)
}
