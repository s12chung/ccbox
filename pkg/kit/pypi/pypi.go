// Package pypi is the release index the pypi: tools list off
package pypi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
)

const (
	// Prefix is the pypi: tools' prefix
	Prefix = "pypi:"
	// BaseURL is PyPI's JSON API
	BaseURL = "https://pypi.org"
)

var (
	// JSONURL is indirected so tests can point at a fixture server
	JSONURL = BaseURL
	client  = &http.Client{Timeout: 30 * time.Second}
)

// Index is PyPI's JSON index
type Index struct{}

// Name is the source's label — its errors' prefix
func (Index) Name() string { return "pypi" }

// List reads the package's release list off the index
func (Index) List(ctx context.Context, tool string) ([]miseutil.VersionEntry, error) {
	body, err := httputil.Body(ctx, client, jsonURL(tool))
	if err != nil {
		return nil, err
	}
	var pkg index
	if err := json.Unmarshal(body, &pkg); err != nil {
		return nil, fmt.Errorf("%s: %w", jsonURL(tool), err)
	}
	return pkg.entries(), nil
}

// Pin keeps the full release — pip's == matches an exact version
func (Index) Pin(version string) string { return version }

func jsonURL(tool string) string {
	return JSONURL + "/pypi/" + strings.TrimPrefix(tool, Prefix) + "/json"
}

// index is a package's PyPI JSON index
type index struct {
	Releases map[string][]file `json:"releases"`
}

// file is one release artifact
type file struct {
	UploadTime string `json:"upload_time_iso_8601"`
}

// entries lists the releases, the yanked ones dropping
func (i index) entries() []miseutil.VersionEntry {
	entries := make([]miseutil.VersionEntry, 0, len(i.Releases))
	for version, files := range i.Releases {
		if len(files) == 0 {
			continue
		}
		e := miseutil.VersionEntry{Version: version}
		if at, err := time.Parse(time.RFC3339, files[0].UploadTime); err == nil {
			e.CreatedAt = &at
		}
		entries = append(entries, e)
	}
	return entries
}
