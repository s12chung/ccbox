package entrypoint

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
)

// Wall probe URLs: api.github.com returns 200 unauthenticated and matches the
// proxy allowlist, so it makes a clean positive probe. Overridable for other allowlists.
const (
	defaultAllowedURL = "https://api.github.com"
	defaultBlockedURL = "https://example.com"
	defaultDirectURL  = "https://1.1.1.1"
)

const (
	allowedURLVar = "WALL_ALLOWED_URL"
	blockedURLVar = "WALL_BLOCKED_URL"
	directURLVar  = "WALL_DIRECT_URL"

	// proxyEnv marks the wall: the container only has the proxy env vars when the
	// wall exists — no http_proxy, no wall to verify (`ccbox --no-proxy` runs on
	// plain bridge networking without them).
	proxyEnv = "http_proxy"

	probeMax = 5 * time.Second
)

// probeWall verifies the egress wall with three curl-parity probes: an allowlisted
// host must be reachable through the proxy, a non-allowlisted host must be rejected
// by it, and bypassing it must have no route at all — proving --internal holds even
// for tools that ignore proxy env vars.
func probeWall() error {
	proxy := probeClient(http.ProxyFromEnvironment)
	direct := probeClient(nil)

	if err := probeAllowed(proxy, envOr(allowedURLVar, defaultAllowedURL)); err != nil {
		return err
	}
	if err := probeBlocked(proxy, envOr(blockedURLVar, defaultBlockedURL)); err != nil {
		return err
	}
	return probeDirect(direct, envOr(directURLVar, defaultDirectURL))
}

// probeAllowed asserts an allowlisted host is reachable through the proxy client.
func probeAllowed(client *http.Client, rawURL string) error {
	if _, err := reachable(client, rawURL); err != nil {
		return fmt.Errorf("allowed host unreachable (%s) — proxy or network is down: %w", rawURL, err)
	}
	return nil
}

// probeBlocked asserts a non-allowlisted host is rejected by the proxy. Failing
// here is the success case and must stay silent.
func probeBlocked(client *http.Client, rawURL string) error {
	if ok, _ := reachable(client, rawURL); ok {
		return fmt.Errorf("blocked host reachable through proxy (%s) — allowlist not enforced", rawURL)
	}
	return nil
}

// probeDirect asserts bypassing the proxy has no route at all, so tools that
// ignore proxy env vars stay isolated.
func probeDirect(client *http.Client, rawURL string) error {
	if ok, _ := reachable(client, rawURL); ok {
		return fmt.Errorf("direct egress works (%s) — network is not --internal", rawURL)
	}
	return nil
}

// probeClient builds a client with a fixed proxy: ProxyFromEnvironment honors the
// container's proxy env vars; nil forces direct dialing — the isolation probe's
// whole point.
func probeClient(proxy func(*http.Request) (*url.URL, error)) *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: proxy}}
}

// reachable GETs rawURL and reports whether it succeeded like curl -f: any
// response below HTTP 400 counts, everything else — transport error, 4xx, 5xx —
// does not.
func reachable(client *http.Client, rawURL string) (bool, error) {
	status, err := getStatusCodeWithTimeout(client, rawURL)
	if err != nil {
		return false, err
	}
	if status >= http.StatusBadRequest {
		return false, fmt.Errorf("http %d", status)
	}
	return true, nil
}

// getStatusCodeWithTimeout GETs rawURL with the probe timeout and returns its
// status code; the response is drained for connection reuse and closed here —
// callers only read the status.
func getStatusCodeWithTimeout(client *http.Client, rawURL string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeMax)
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
