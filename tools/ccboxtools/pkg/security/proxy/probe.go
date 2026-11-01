package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Proxy probe URLs: api.github.com returns 200 unauthenticated and matches the
// proxy allowlist, so it makes a clean positive probe.
const (
	defaultAllowedURL = "https://api.github.com"
	defaultBlockedURL = "https://example.com"
	defaultDirectURL  = "https://1.1.1.1"
)

// The probes' targets — vars so tests aim them at stubs.
var (
	allowedURL = defaultAllowedURL
	blockedURL = defaultBlockedURL
	directURL  = defaultDirectURL
)

const (
	// proxyEnv marks the proxy: the env vars exist only when the egress wall
	// does (`ccbox --no-proxy` runs without them)
	proxyEnv = "http_proxy"

	probeMax       = 5 * time.Second
	directProbeMax = time.Second
)

// probeProxy verifies the egress wall: an allowlisted host is reachable
// through the proxy, a blocked host is not, and direct egress has no route —
// any failure is the rejection probes' pass.
//
// A listener sweep of pkginfo.ProxyAddr and the container's dynamic docker IP
// could run here — rejected: the egress wall does not safeguard against
// processes binding to those addresses anyway.
func probeProxy(ctx context.Context) error {
	if err := verifyShareRO(); err != nil {
		return err
	}
	proxy := probeClient(proxyFrom, probeMax)
	direct := probeClient(nil, directProbeMax)

	if _, err := reachable(ctx, proxy, allowedURL); err != nil {
		return fmt.Errorf("allowed host unreachable (%s) — proxy or network is down: %w", allowedURL, err)
	}
	if ok, _ := reachable(ctx, proxy, blockedURL); ok {
		return fmt.Errorf("blocked host reachable through proxy (%s) — allowlist not enforced", blockedURL)
	}
	if ok, _ := reachable(ctx, direct, directURL); ok {
		return fmt.Errorf("direct egress works (%s) — network is not --internal", directURL)
	}
	return nil
}

// proxyFrom pins the probes to the egress wall's own address — the probes
// verify the egress wall itself, not whatever http_proxy points at
var proxyFrom = func(*http.Request) (*url.URL, error) {
	return &url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", pkginfo.ProxyAddr, pkginfo.ProxyPort)}, nil
}

// probeClient builds a client with a fixed proxy: ProxyFromEnvironment honors
// the proxy env vars; nil dials direct. Timeout bounds the whole exchange.
func probeClient(proxy func(*http.Request) (*url.URL, error), timeout time.Duration) *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: proxy}, Timeout: timeout}
}

// reachable GETs rawURL and reports success like curl -f: any response below
// HTTP 400 counts.
func reachable(ctx context.Context, client *http.Client, rawURL string) (bool, error) {
	status, err := getStatusCode(ctx, client, rawURL)
	if err != nil {
		return false, err
	}
	if status >= http.StatusBadRequest {
		return false, fmt.Errorf("http %d", status)
	}
	return true, nil
}

// getStatusCode GETs rawURL and returns its status code; the client's Timeout
// bounds it.
func getStatusCode(ctx context.Context, client *http.Client, rawURL string) (int, error) {
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

// The share pin's probes — vars so tests aim them at stubs.
var (
	probeShareRO = mkdirInShare
	openAllow    = openAllowFile
)

// verifyShareRO proves the share's read-only pin
func verifyShareRO() error {
	if err := roVerdict(pkginfo.ProxyMount, probeShareRO()); err != nil {
		return err
	}
	return roVerdict(pkginfo.ProxyAllowPath, openAllow())
}

// roVerdict passes only EROFS: success is the pin gone, any other errno is
// unexplained.
func roVerdict(path string, err error) error {
	switch {
	case err == nil:
		return fmt.Errorf("%s is writable — its read-only pin is gone", path)
	case !errors.Is(err, syscall.EROFS):
		return fmt.Errorf("verify share ro: %w", err)
	}
	return nil
}

func mkdirInShare() error {
	// the 0777 dir makes DAC vacuous, so only the mount can refuse.
	dir, err := os.MkdirTemp(pkginfo.ProxyMount, "ro-probe-")
	if err == nil {
		log.WarnErr("remove share probe", os.Remove(dir))
	}
	return err
}

func openAllowFile() error {
	// with root, returns EACCES whether the mount is ro or rw
	f, err := os.OpenFile(pkginfo.ProxyAllowPath, os.O_WRONLY, 0)
	if err == nil {
		log.WarnErr("close allow file", f.Close())
	}
	return err
}
