package mise

import (
	"encoding/json"
	"strings"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
)

const (
	pypiPrefix  = "pypi:"
	pypiBaseURL = "https://pypi.org"

	aquaPrefix  = "aqua:"
	aquaBaseURL = "https://api.github.com"
)

// pypiURL and aquaURL are indirected so tests can point at a fixture server
var (
	pypiURL = pypiBaseURL
	aquaURL = aquaBaseURL
)

// init registers the release sources beside the versions service's default —
// each backend prefix its own index
func init() {
	miseutil.RegisterSource(pypiPrefix, miseutil.IndexSource(pypiIndex{}, nil))
	miseutil.RegisterSource(aquaPrefix, miseutil.IndexSource(aquaIndex{}, miseutil.MinorPin))
}

// pypiIndex is PyPI's JSON index as the pypi: tools' source — a nil pin rule
// keeps the full release, pip's == matching an exact version
type pypiIndex struct{}

// Label is the source's label — its errors' prefix
func (pypiIndex) Label() string { return "pypi" }

// ListURL is the package's index path
func (pypiIndex) ListURL(tool string) string {
	return pypiURL + "/pypi/" + strings.TrimPrefix(tool, pypiPrefix) + "/json"
}

// BodyToEntries lists the index's releases, the yanked ones dropping
func (pypiIndex) BodyToEntries(body []byte) ([]miseutil.VersionEntry, error) {
	var decoded pypiReleases
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	entries := make([]miseutil.VersionEntry, 0, len(decoded.Releases))
	for version, files := range decoded.Releases {
		if len(files) == 0 { // yanked — nothing to install
			continue
		}
		entries = append(entries, miseutil.Entry(version, files[0].UploadTime))
	}
	return entries, nil
}

// pypiReleases is a package's PyPI JSON index
type pypiReleases struct {
	Releases map[string][]pypiFile `json:"releases"`
}

// pypiFile is one release artifact
type pypiFile struct {
	UploadTime string `json:"upload_time_iso_8601"`
}

// aquaIndex is the aqua: tools' release index as their source — the pin
// truncates to its minor, the config's depth off the versions service
type aquaIndex struct{}

// Label is the source's label — its errors' prefix
func (aquaIndex) Label() string { return "aqua" }

// ListURL is the repo's releases path
func (aquaIndex) ListURL(tool string) string {
	return aquaURL + "/repos/" + strings.TrimPrefix(tool, aquaPrefix) + "/releases"
}

// BodyToEntries lists the repo's releases, tags reading as versions
func (aquaIndex) BodyToEntries(body []byte) ([]miseutil.VersionEntry, error) {
	var decoded []aquaRelease
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	entries := make([]miseutil.VersionEntry, 0, len(decoded))
	for _, release := range decoded {
		entry := miseutil.Entry(strings.TrimPrefix(release.TagName, "v"), release.PublishedAt)
		if release.PreRelease {
			entry.PreRelease = &release.PreRelease
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// aquaRelease is one repo release
type aquaRelease struct {
	TagName     string `json:"tag_name"`
	PreRelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}
