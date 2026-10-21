package mise

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
)

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
		assert.Equal(t, "82.0.1", pin)
	})

	t.Run("a fetch error carries the pypi source's label", func(t *testing.T) {
		stubPyPI(t, nil)

		_, err := miseutil.LatestVersion(t.Context(), "pypi:bogus")

		require.ErrorContains(t, err, "pypi: pypi:bogus:")
		require.ErrorContains(t, err, "404 Not Found")
	})
}
