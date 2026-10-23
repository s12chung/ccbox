package miseutil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexSource(t *testing.T) {
	t.Run("the label, list url and pin rule bind", func(t *testing.T) {
		stubVersions(t, map[string]string{"/releases.json": `{"releases":{"1.2.3":{"at":"2026-01-01T00:00:00Z"}}}`})
		src := IndexSource(stubIndex{}, minorPin)

		assert.Equal(t, "stub", src.Name())

		at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		entries, err := src.List(t.Context(), "wheel")
		require.NoError(t, err)
		assert.Equal(t, []VersionEntry{{Version: "1.2.3", CreatedAt: &at}}, entries)

		assert.Equal(t, "1.2", src.Pin("1.2.3"))
	})

	t.Run("a decode error carries the list url", func(t *testing.T) {
		stubVersions(t, map[string]string{"/releases.json": `{"releases":`})
		src := IndexSource(stubIndex{}, nil)

		_, err := src.List(t.Context(), "wheel")

		require.ErrorContains(t, err, "/releases.json")
	})

	t.Run("a fetch error passes through", func(t *testing.T) {
		stubVersions(t, nil)
		src := IndexSource(stubIndex{}, nil)

		_, err := src.List(t.Context(), "wheel")

		require.ErrorContains(t, err, "404 Not Found")
	})

	t.Run("a nil pin rule keeps the full release", func(t *testing.T) {
		src := IndexSource(stubIndex{}, nil)

		assert.Equal(t, "1.2.3", src.Pin("1.2.3"))
	})
}

// stubIndex is an IndexSource test's index — its body one dated release
type stubIndex struct{}

func (stubIndex) Label() string { return "stub" }

func (stubIndex) ListURL(string) string { return VersionsURL + "/releases.json" }

func (stubIndex) BodyToEntries(body []byte) ([]VersionEntry, error) {
	var decoded struct {
		Releases map[string]struct {
			At string `json:"at"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	entries := make([]VersionEntry, 0, len(decoded.Releases))
	for version, release := range decoded.Releases {
		entries = append(entries, Entry(version, release.At))
	}
	return entries, nil
}

func TestEntry(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, &at, Entry("1.14.0", at.Format(time.RFC3339)).CreatedAt)
	assert.Nil(t, Entry("1.14.0", "not-a-time").CreatedAt, "an unreadable time reads undated")
}
