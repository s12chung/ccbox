package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Proxy probe URLs: api.github.com returns 200 unauthenticated and matches the
// proxy allowlist, so it makes a clean positive probe. Overridable for other allowlists.
const (
	defaultAllowedURL = "https://api.github.com"
	defaultBlockedURL = "https://example.com"
	defaultDirectURL  = "https://1.1.1.1"
)

const (
	allowedURLVar = "PROXY_ALLOWED_URL"
	blockedURLVar = "PROXY_BLOCKED_URL"
	directURLVar  = "PROXY_DIRECT_URL"

	// proxyEnv marks the proxy: the env vars exist only when the egress wall
	// does (`ccbox --no-proxy` runs without them)
	proxyEnv = "http_proxy"

	probeMax = 5 * time.Second
)

// probeProxy verifies the egress wall: an allowlisted host is reachable
// through the proxy, a blocked host is not, and direct egress has no route —
// any failure is the rejection probes' pass.
func probeProxy(ctx context.Context) error {
	proxy := probeClient(http.ProxyFromEnvironment)
	direct := probeClient(nil)

	if _, err := reachable(ctx, proxy, envOr(allowedURLVar, defaultAllowedURL)); err != nil {
		return fmt.Errorf("allowed host unreachable (%s) — proxy or network is down: %w", envOr(allowedURLVar, defaultAllowedURL), err)
	}
	if ok, _ := reachable(ctx, proxy, envOr(blockedURLVar, defaultBlockedURL)); ok {
		return fmt.Errorf("blocked host reachable through proxy (%s) — allowlist not enforced", envOr(blockedURLVar, defaultBlockedURL))
	}
	if ok, _ := reachable(ctx, direct, envOr(directURLVar, defaultDirectURL)); ok {
		return fmt.Errorf("direct egress works (%s) — network is not --internal", envOr(directURLVar, defaultDirectURL))
	}
	return nil
}

// probeClient builds a client with a fixed proxy: ProxyFromEnvironment honors
// the proxy env vars; nil dials direct.
func probeClient(proxy func(*http.Request) (*url.URL, error)) *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: proxy}}
}

// reachable GETs rawURL and reports success like curl -f: any response below
// HTTP 400 counts.
func reachable(ctx context.Context, client *http.Client, rawURL string) (bool, error) {
	status, err := getStatusCodeWithTimeout(ctx, client, rawURL)
	if err != nil {
		return false, err
	}
	if status >= http.StatusBadRequest {
		return false, fmt.Errorf("http %d", status)
	}
	return true, nil
}

// getStatusCodeWithTimeout GETs rawURL with the probe timeout and returns its
// status code.
func getStatusCodeWithTimeout(ctx context.Context, client *http.Client, rawURL string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, probeMax)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { log.WarnErr("close response", resp.Body.Close()) }()

	_, err = io.Copy(io.Discard, resp.Body) // drain for connection reuse
	log.WarnErr("drain response", err)

	return resp.StatusCode, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
