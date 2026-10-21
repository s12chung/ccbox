// Package miseutil resolves tools' latest installable releases — the same rule
// the lock's own mise resolves by — to the pin each source binds
package miseutil

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// releaseAge is mise's minimum_release_age default, mirrored here so a pin
// never races the lock
const releaseAge = 24 * time.Hour

var versionsClient = &http.Client{Timeout: 30 * time.Second}

// LatestVersion resolves the tool's latest installable release, pinned as its
// source binds it
func LatestVersion(ctx context.Context, tool string) (string, error) {
	latest, err := newestInstallable(ctx, tool)
	if err != nil {
		return "", err
	}
	return sourceFor(tool).Pin(latest), nil
}

// Source reads a tool's release list
type Source interface {
	// Name is the source's label — its errors' prefix
	Name() string
	List(ctx context.Context, tool string) ([]VersionEntry, error)
	// Pin truncates a release to the pin the config writes — the source's
	// own lock decides its depth
	Pin(version string) string
}

var sources = map[string]Source{}

// sourceFor picks the tool's source — its longest registered prefix's
func sourceFor(tool string) Source {
	prefixes := slices.Sorted(maps.Keys(sources))
	for _, prefix := range slices.Backward(prefixes) {
		if strings.HasPrefix(tool, prefix) {
			return sources[prefix]
		}
	}
	return defaultSource
}

// RegisterSource registers src as the source of prefix's tools
func RegisterSource(prefix string, src Source) { sources[prefix] = src }

func newestInstallable(ctx context.Context, tool string) (string, error) {
	src := sourceFor(tool)
	entries, err := src.List(ctx, tool)
	if err != nil {
		return "", fmt.Errorf("%s: %s: %w", src.Name(), tool, err)
	}
	for i := range entries {
		entries[i].segments = parseSegments(entries[i].Version)
	}
	slices.SortFunc(entries, VersionEntry.compare)
	for _, e := range entries {
		if e.isInstallable() {
			return e.Version, nil
		}
	}
	return "", fmt.Errorf("%s: %s: no installable release in the list", src.Name(), tool)
}

// VersionEntry is one release in a source's list
type VersionEntry struct {
	Version    string     `toml:"-"`
	CreatedAt  *time.Time `toml:"created_at"`
	PreRelease *bool      `toml:"prerelease"`

	segments []int // nil marks an unstable version
}

// isInstallable reports the release is one the lock's mise would resolve
func (e VersionEntry) isInstallable() bool {
	return !e.isPreRelease() && e.isStable() && !e.isYoungerThan(time.Now().Add(-releaseAge))
}

func (e VersionEntry) isPreRelease() bool {
	return e.PreRelease != nil && *e.PreRelease
}

// isYoungerThan reports the release landed after cutoff — undated reads as aged
func (e VersionEntry) isYoungerThan(cutoff time.Time) bool {
	return e.CreatedAt != nil && e.CreatedAt.After(cutoff)
}

// isStable reports the version is numeric — some lists carry unflagged
// variant lines (jruby-*, *-dev) that chart above the real releases
func (e VersionEntry) isStable() bool { return e.segments != nil }

// compare orders the entries by version, newest first
func (e VersionEntry) compare(other VersionEntry) int {
	return -slices.Compare(e.segments, other.segments)
}

// parseSegments splits the version's dot-segments — nil when any isn't numeric
func parseSegments(version string) []int {
	parts := strings.Split(version, ".")
	segments := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		segments[i] = n
	}
	return segments
}
