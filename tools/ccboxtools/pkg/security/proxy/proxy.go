// Package proxy is the egress wall's in-container mechanism: a pinned child
// netns whose only route out leads through tinyproxy — built at boot, proven by
// the proxy child, entered by every session through the drop chain.
package proxy

import "os"

// Proxied reports the proxy env's presence — the env vars exist only when the
// egress wall does (`ccbox --no-proxy` runs without them).
func Proxied() bool { return os.Getenv(proxyEnv) != "" }
