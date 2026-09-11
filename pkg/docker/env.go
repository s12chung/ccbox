package docker

import "sort"

// envString renders env as KEY=VALUE in sorted-key order for a deterministic spec, with the
// wall's proxy vars appended last — so they override any user-supplied proxy setting.
func envString(env map[string]string, noProxy bool) []string {
	renderedEnv := sortedEnv(env)
	if noProxy {
		return renderedEnv
	}
	return append(renderedEnv, []string{
		"http_proxy=http://" + egressName + ":" + proxyPort,
		"https_proxy=http://" + egressName + ":" + proxyPort,
		// Loopback never leaves the container, so route it direct — else local dev servers
		// and browsers hit the wall and get refused.
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
