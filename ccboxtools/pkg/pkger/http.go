package pkger

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// httpGet does a context-bound GET on the default client, which honors proxy
// env (the egress wall) via ProxyFromEnvironment.
func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// httpBody GETs the url, failing on non-200, and reads its body.
func httpBody(url string) ([]byte, error) {
	resp, err := httpGetOK(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	return io.ReadAll(resp.Body)
}

// httpGetOK GETs the url, failing on non-200; the caller closes the body.
func httpGetOK(url string) (*http.Response, error) {
	resp, err := httpGet(context.Background(), url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // failing is ok
		return nil, fmt.Errorf("%s: %s", resp.Request.URL, resp.Status)
	}
	return resp, nil
}
