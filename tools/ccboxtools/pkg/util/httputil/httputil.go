// Package httputil does context-bound GETs on the default client, which honors
// proxy env (the egress wall) via ProxyFromEnvironment.
package httputil

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// GetOK GETs the url through the client — nil is the default — failing on
// non-200; the caller closes the body.
func GetOK(client *http.Client, url string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
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
func Body(client *http.Client, url string) ([]byte, error) {
	resp, err := GetOK(client, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	return io.ReadAll(resp.Body)
}
