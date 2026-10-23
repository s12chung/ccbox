package miseutil

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
)

var indexClient = &http.Client{Timeout: 30 * time.Second}

// Index is a release index served over HTTP — its impl decodes its own body,
// the fetch and Source binding staying here
type Index interface {
	// Label is the source's label — its errors' prefix
	Label() string
	// ListURL is the tool's list url
	ListURL(tool string) string
	// BodyToEntries maps the fetched body to its entries
	BodyToEntries(body []byte) ([]VersionEntry, error)
}

// IndexSource binds an index as a Source — a nil pin rule keeps the full
// release
func IndexSource(index Index, pinRule func(version string) string) Source {
	return indexSource{index, pinRule}
}

// indexSource is an Index as its Source — the fetch's only machinery
type indexSource struct {
	index   Index
	pinRule func(version string) string
}

// Name is the source's label — its errors' prefix
func (s indexSource) Name() string { return s.index.Label() }

// List fetches the tool's list body off to its entries
func (s indexSource) List(ctx context.Context, tool string) ([]VersionEntry, error) {
	url := s.index.ListURL(tool)
	body, err := httputil.Body(ctx, indexClient, url)
	if err != nil {
		return nil, err
	}
	entries, err := s.index.BodyToEntries(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return entries, nil
}

// Pin truncates a release to the pin the config writes — a nil rule keeps the
// full release
func (s indexSource) Pin(version string) string {
	if s.pinRule == nil {
		return version
	}
	return s.pinRule(version)
}

// Entry is a release entry off its RFC3339 timestamp — an unreadable time reads undated
func Entry(version, at string) VersionEntry {
	entry := VersionEntry{Version: version}
	if createdAt, err := time.Parse(time.RFC3339, at); err == nil {
		entry.CreatedAt = &createdAt
	}
	return entry
}
