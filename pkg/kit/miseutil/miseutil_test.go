package miseutil

import (
	"context"
	"slices"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/must"
)

// renderTmpl executes the fixture tmpl — its static shapes never fail
func renderTmpl(tmpl *template.Template, data any) string {
	var b strings.Builder
	must.Do(tmpl.Execute(&b, data))
	return b.String()
}

func TestLatestVersion(t *testing.T) {
	aged := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stubVersions(t, map[string]string{"/data/node.toml": versionList(map[string]time.Time{"26.12.0": aged})})
	RegisterSource("stub:", stubSource{})
	t.Cleanup(func() { delete(sources, "stub:") })

	node, err := LatestVersion(t.Context(), "node")
	require.NoError(t, err)
	assert.Equal(t, "26.12", node)

	wheel, err := LatestVersion(t.Context(), "stub:wheel")
	require.NoError(t, err)
	assert.Equal(t, "1.0.1", wheel,
		"a prefixed tool resolves off its registered source, every other off the versions service — each pinned as its source binds")
}

func TestLatestVersion_Errors(t *testing.T) {
	stubVersions(t, nil)

	_, err := LatestVersion(t.Context(), "bogus")

	require.ErrorContains(t, err, "mise: bogus:")
	require.ErrorContains(t, err, "404 Not Found")
}

// stubSource stands in for a registered backend — its one release
type stubSource struct{}

func (stubSource) Name() string { return "stub" }

func (stubSource) List(context.Context, string) ([]VersionEntry, error) {
	return []VersionEntry{{Version: "1.0.1"}}, nil
}

func (stubSource) Pin(version string) string { return version }

func TestNewestInstallable(t *testing.T) {
	t.Run("the list's newest installable release, everything the lock would skip dropped", func(t *testing.T) {
		versions := map[string]time.Time{
			"jruby-1.7.2":    time.Date(2025, 12, 18, 0, 38, 7, 0, time.UTC), // unflagged variant line
			"4.0.7":          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			"3.99.0":         time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), // newer date, lower branch
			"4.1.0-preview1": time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),  // flagged prerelease
			"4.1-dev":        time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),  // a dev line above its stable
			"4.2.0":          time.Now().Add(time.Hour),                    // younger than the release age
		}
		stubVersions(t, map[string]string{"/data/ruby.toml": versionList(versions)})

		v, err := newestInstallable(t.Context(), "ruby")

		require.NoError(t, err)
		assert.Equal(t, "4.0.7", v)
	})

	t.Run("a list with nothing installable errors, carrying source and tool", func(t *testing.T) {
		list := "[versions]\n\"1.14.0-rc1\" = { created_at = 2026-01-01T00:00:00.000Z, prerelease = true }\n"
		stubVersions(t, map[string]string{"/data/bats.toml": list})

		_, err := newestInstallable(t.Context(), "bats")

		require.ErrorContains(t, err, "mise: bats: no installable release")
	})
}

func TestParseSegments(t *testing.T) {
	assert.Equal(t, []int{1, 14, 0}, parseSegments("1.14.0"))
	assert.Equal(t, []int{4}, parseSegments("4"))
	assert.Nil(t, parseSegments("4.1-dev"), "unstable — every dot-segment must parse numeric")
	assert.Nil(t, parseSegments("3.16.0a1"))
	assert.Nil(t, parseSegments("4.1.0-preview1"))
	assert.Nil(t, parseSegments("jruby-9.0.0.0.pre1"))
}

func TestVersionEntry_Compare(t *testing.T) {
	entry := func(v string) VersionEntry { return VersionEntry{Version: v, segments: parseSegments(v)} }
	entries := []VersionEntry{entry("4.0.7"), entry("3.99.0"), entry("4.1.0"), entry("4")}

	slices.SortFunc(entries, VersionEntry.compare)

	ordered := make([]string, len(entries))
	for i, e := range entries {
		ordered[i] = e.Version
	}
	assert.Equal(t, []string{"4.1.0", "4.0.7", "4", "3.99.0"}, ordered,
		"newest first — a bare major charts as its zeroed minor")
}
