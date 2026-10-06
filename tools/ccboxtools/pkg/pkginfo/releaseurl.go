package pkginfo

import (
	"regexp"
	"runtime"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// ReleaseURL installs from a URL whose body resolves the version: a bare-version
// endpoint, or a structured document queried with a jq selector (JQSchema).
type ReleaseURL struct {
	// URL's literal $arch is substituted with the running arch's vendor name
	// (ArchedURL)
	URL              string            `json:"url"               yaml:"url"`
	DownloadTemplate *DownloadTemplate `json:"download_template" yaml:"download_template"`
	JQSchema         *JQSchema         `json:"jq_schema"         yaml:"jq_schema"`
	Artifact         *Artifact         `json:"artifact"          yaml:"artifact"`
}

// ArchedURL returns URL with $arch substituted by the running arch's vendor name
// (x64/arm64); a url without $arch passes through.
func (r ReleaseURL) ArchedURL() string {
	arch, ok := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if !ok {
		return r.URL
	}
	return strings.ReplaceAll(r.URL, "$arch", arch)
}

// DownloadTemplate carries the per-arch download URL templates; a literal $version
// is substituted at download.
type DownloadTemplate struct {
	X64URL   string `json:"x64_url"   yaml:"x64_url"`
	Arm64URL string `json:"arm64_url" yaml:"arm64_url"`
}

// JQSchema queries the manifest mode's document per its Format: Version
// resolves the version, download_url names the file to download, and sha512
// pins its hash (base64 std) for verification at download.
type JQSchema struct {
	// Format names the document's parser: yaml or json
	Format      string `json:"format"       yaml:"format"`
	Version     string `json:"version"      yaml:"version"`
	DownloadURL string `json:"download_url" yaml:"download_url"`
	Sha512      string `json:"sha512"       yaml:"sha512"`
}

// Artifact is a packaged download: Type names the unpacker (deb now; zip etc.
// later), RelBin is the executable's path inside the extracted tree.
type Artifact struct {
	Type   string `json:"type"    yaml:"type"`
	RelBin string `json:"rel_bin" yaml:"rel_bin"`
}

func init() {
	https := rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	firm.MustRegisterType(firm.NewDefinition[ReleaseURL]().
		ValidatesSelf(rule.OneNotNil{Fields: []string{"DownloadTemplate", "JQSchema"}}).
		Validates(firm.RuleMap{
			"URL":              {https},
			"DownloadTemplate": {firm.Backed()},
			"JQSchema":         {firm.Backed()},
			"Artifact":         {firm.Backed()},
		}))
	firm.MustRegisterType(firm.NewDefinition[DownloadTemplate]().Validates(firm.RuleMap{
		"X64URL":   {https},
		"Arm64URL": {https},
	}))
	firm.MustRegisterType(firm.NewDefinition[JQSchema]().Validates(firm.RuleMap{
		"Format":      {rule.OneOf[string]{Values: []string{"yaml", "json"}}},
		"Version":     {jqExpr{}, rule.Present{}},
		"DownloadURL": {jqExpr{}, rule.Present{}},
		"Sha512":      {jqExpr{}},
	}))
	firm.MustRegisterType(firm.NewDefinition[Artifact]().Validates(firm.RuleMap{
		"Type":   {rule.OneOf[string]{Values: []string{"deb"}}},
		"RelBin": {rule.Present{}},
	}))
}
