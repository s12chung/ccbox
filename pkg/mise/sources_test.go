package mise

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
)

// TestPypiListURL pins the tool's index path
func TestPypiListURL(t *testing.T) {
	assert.Equal(t, pypiBaseURL+"/pypi/setuptools/json", pypiIndex{}.ListURL("pypi:setuptools"))
}

func TestPypiBodyToEntries(t *testing.T) {
	t.Run("reads the package's releases off its index", func(t *testing.T) {
		agedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		entries, err := pypiIndex{}.BodyToEntries([]byte(`{"releases":{"0.48.0":[{"upload_time_iso_8601":"2026-01-01T00:00:00Z"}]}}`))

		require.NoError(t, err)
		assert.Equal(t, []miseutil.VersionEntry{{Version: "0.48.0", CreatedAt: &agedAt}}, entries)
	})

	t.Run("the yanked releases drop, each remaining one dated by its first file", func(t *testing.T) {
		at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		body := `{"releases":{` +
			`"82.0.1":[{"upload_time_iso_8601":"` + at.Format(time.RFC3339) + `"}],` +
			`"82.0.0":[],` + // its files all deleted
			`"81.3.0":[{"upload_time_iso_8601":"bogus"}]}}` // an undatable file

		entries, err := pypiIndex{}.BodyToEntries([]byte(body))

		require.NoError(t, err)
		require.Len(t, entries, 2, "an empty release — nothing to install — drops")
		byVersion := map[string]miseutil.VersionEntry{}
		for _, e := range entries {
			byVersion[e.Version] = e
		}
		assert.Equal(t, &at, byVersion["82.0.1"].CreatedAt, "dated by its first file")
		assert.Nil(t, byVersion["81.3.0"].CreatedAt, "an undatable file reads undated — aged")
	})

	t.Run("a malformed body errors", func(t *testing.T) {
		_, err := pypiIndex{}.BodyToEntries([]byte(`{"releases":`))

		require.Error(t, err)
	})
}

// TestPypiRegistration covers the init wiring
func TestPypiRegistration(t *testing.T) {
	t.Run("a pypi: tool resolves off PyPI's index", func(t *testing.T) {
		aged := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		stubPyPI(t, map[string]string{"/pypi/setuptools/json": fmt.Sprintf(`{"releases":{`+
			`"84.0.0":[{"upload_time_iso_8601":%q}],`+ // younger than the release age
			`"83.0.0rc1":[{"upload_time_iso_8601":%q}],`+ // an unflagged prerelease
			`"82.0.1":[{"upload_time_iso_8601":%q}]}}`,
			time.Now().Add(time.Hour).Format(time.RFC3339), aged, aged,
		)})

		pin, err := miseutil.LatestVersion(t.Context(), "pypi:setuptools")

		require.NoError(t, err)
		assert.Equal(t, "82.0.1", pin, "the full release — the nil pin rule keeps it untruncated")
	})

	t.Run("a fetch error carries the pypi source's label", func(t *testing.T) {
		stubPyPI(t, nil)

		_, err := miseutil.LatestVersion(t.Context(), "pypi:bogus")

		require.ErrorContains(t, err, "pypi: pypi:bogus:")
		require.ErrorContains(t, err, "404 Not Found")
	})
}

// TestAquaListURL pins the repo's releases path
func TestAquaListURL(t *testing.T) {
	assert.Equal(t, aquaBaseURL+"/repos/quarylabs/sqruff/releases", aquaIndex{}.ListURL("aqua:quarylabs/sqruff"))
}

func TestAquaBodyToEntries(t *testing.T) {
	t.Run("tags read as versions, dated off their publish time", func(t *testing.T) {
		at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		entries, err := aquaIndex{}.BodyToEntries([]byte(fmt.Sprintf(
			`[{"tag_name":"v0.43.0","published_at":%q},{"tag_name":"v1.2","published_at":%q}]`,
			at.Format(time.RFC3339), at.Format(time.RFC3339))))

		require.NoError(t, err)
		assert.Equal(t, []miseutil.VersionEntry{
			{Version: "0.43.0", CreatedAt: &at},
			{Version: "1.2", CreatedAt: &at},
		}, entries)
	})

	t.Run("a flagged prerelease stays flagged", func(t *testing.T) {
		flag := true

		entries, err := aquaIndex{}.BodyToEntries([]byte(`[{"tag_name":"v1.0.0-rc1","prerelease":true}]`))

		require.NoError(t, err)
		assert.Equal(t, []miseutil.VersionEntry{{Version: "1.0.0-rc1", PreRelease: &flag}}, entries)
	})

	t.Run("a malformed body errors", func(t *testing.T) {
		_, err := aquaIndex{}.BodyToEntries([]byte(`[`))

		require.Error(t, err)
	})
}

// TestAquaRegistration covers the init wiring
func TestAquaRegistration(t *testing.T) {
	t.Run("an aqua: tool resolves off its repo's releases", func(t *testing.T) {
		aged := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		stubAqua(t, map[string]string{"/repos/quarylabs/sqruff/releases": fmt.Sprintf(
			`[{"tag_name":"v0.43.0","published_at":%q},`+ // younger than the release age
				`{"tag_name":"v0.42.0","published_at":%q}]`,
			time.Now().Add(time.Hour).Format(time.RFC3339), aged)})

		pin, err := miseutil.LatestVersion(t.Context(), "aqua:quarylabs/sqruff")

		require.NoError(t, err)
		assert.Equal(t, "0.42", pin, "the pin truncates to its minor")
	})

	t.Run("a fetch error carries the aqua source's label", func(t *testing.T) {
		stubAqua(t, nil)

		_, err := miseutil.LatestVersion(t.Context(), "aqua:bogus")

		require.ErrorContains(t, err, "aqua: aqua:bogus:")
		require.ErrorContains(t, err, "404 Not Found")
	})
}
