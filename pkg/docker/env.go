package docker

import (
	"maps"
	"sort"

	"github.com/s12chung/ccbox/pkg/harness"
)

// envString renders the container's environment as KEY=VALUE
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

	// Later entries override when conflicting
	return append(append(cliEnv(o), sortedEnv(o.Env)...), base...)
}

// cliEnv renders the selected CLI's required env
func cliEnv(o RunOptions) []string {
	cli := harness.MustFor(o.CLI)
	env := maps.Clone(cli.Env)
	if env == nil {
		env = map[string]string{}
	}
	return sortedEnv(env)
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
