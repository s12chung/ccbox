// Package hubkit resolves images' latest tags off Docker Hub's registry API
package hubkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
)

var (
	// tokenURLFormat and listURLFormat are indirected so tests can point at a fixture server
	tokenURLFormat = "https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull" //nolint:gosec // no credential, the endpoint
	listURLFormat  = "https://registry-1.docker.io/v2/%s/tags/list"
	client         = &http.Client{Timeout: 30 * time.Second}
)

// LatestTag resolves the repo's newest tag the matchRegexp names — newest meaning
// the lexically greatest name
func LatestTag(ctx context.Context, repo string, matchRegexp *regexp.Regexp) (string, error) {
	token, err := pullToken(ctx, repo)
	if err != nil {
		return "", err
	}
	tags, err := listTags(ctx, repo, token)
	if err != nil {
		return "", err
	}
	var latest string
	for _, tag := range tags {
		if matchRegexp.MatchString(tag) && tag > latest {
			latest = tag
		}
	}
	if latest == "" {
		return "", fmt.Errorf("hub: %s: the listing has no tag matching %s", repo, matchRegexp)
	}
	return latest, nil
}

// pullToken takes the repo's anonymous pull token off the token service
func pullToken(ctx context.Context, repo string) (string, error) {
	body, err := httputil.Body(ctx, client, fmt.Sprintf(tokenURLFormat, repo), nil)
	if err != nil {
		return "", err
	}
	var token struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.Token, nil
}

// listTags fetches the repo's full tag list — the listing arrives whole, no
// paging with the n param unset
func listTags(ctx context.Context, repo, token string) ([]string, error) {
	body, err := httputil.Body(ctx, client, fmt.Sprintf(listURLFormat, repo),
		http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		return nil, err
	}
	var listing struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, err
	}
	return listing.Tags, nil
}
