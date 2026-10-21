// Package httputil does context-bound GETs on the default client, which honors
// proxy env (the egress wall) via ProxyFromEnvironment.
package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Get is the client's GET — nil is the default — context-bound
func Get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// GetOK GETs the url through the client — nil is the default — failing on
// non-200; the caller closes the body.
func GetOK(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	resp, err := Get(ctx, client, url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // failing is ok
		return nil, fmt.Errorf("%s: %s", resp.Request.URL, resp.Status)
	}
	return resp, nil
}

// Body GETs the url through the client — nil is the default — and reads its body.
func Body(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	resp, err := GetOK(ctx, client, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	return io.ReadAll(resp.Body)
}
