package miseutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
)

// VersionsBaseURL is mise's versions service
const VersionsBaseURL = "https://mise-versions.jdx.dev"

// VersionsURL is indirected so tests can point at a fixture server
var VersionsURL = VersionsBaseURL

var defaultSource Source = versionsSource{}

type versionsSource struct{}

func (versionsSource) Name() string { return "mise" }

func (versionsSource) List(ctx context.Context, tool string) ([]VersionEntry, error) {
	body, err := httputil.Body(ctx, versionsClient, toolURL(tool))
	if err != nil {
		return nil, err
	}
	var file versionListFile
	if err := toml.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("%s: %w", toolURL(tool), err)
	}
	return file.entries(), nil
}

// Pin truncates to the minor — mise's own lock pins the patch
func (versionsSource) Pin(version string) string { return minorPin(version) }

// toolURL is the tool's list URL — backend prefixes dropped
func toolURL(tool string) string {
	if i := strings.LastIndex(tool, ":"); i >= 0 {
		tool = tool[i+1:]
	}
	return VersionsURL + "/data/" + tool + ".toml"
}

// minorPin truncates a version to its minor
func minorPin(version string) string {
	parts := strings.SplitN(version, ".", 3)
	return strings.Join(parts[:min(len(parts), 2)], ".")
}

// versionListFile is a tool's list file on the versions service
type versionListFile struct {
	Versions map[string]VersionEntry `toml:"versions"`
}

func (f versionListFile) entries() []VersionEntry {
	entries := make([]VersionEntry, 0, len(f.Versions))
	for version, entry := range f.Versions {
		entry.Version = version
		entries = append(entries, entry)
	}
	return entries
}
