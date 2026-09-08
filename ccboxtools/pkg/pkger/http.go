package pkger

import (
	"context"
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
