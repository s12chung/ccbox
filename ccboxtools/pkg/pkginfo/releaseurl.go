package pkginfo

import (
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// ReleaseURL installs from a URL whose body is a bare version. The per-arch
// download URL templates (DownloadTemplate) carry a literal $version,
// substituted at download.
type ReleaseURL struct {
	URL              string            `json:"url"               yaml:"url"`
	DownloadTemplate *DownloadTemplate `json:"download_template" yaml:"download_template"`
}

// DownloadTemplate carries the per-arch download URL templates; a literal $version
// is substituted at download.
type DownloadTemplate struct {
	X64URL   string `json:"x64_url"   yaml:"x64_url"`
	Arm64URL string `json:"arm64_url" yaml:"arm64_url"`
}

func init() {
	// https endpoints; download templates may carry a literal $version
	https := rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	firm.MustRegisterType(firm.NewDefinition[ReleaseURL]().
		NotNil("DownloadTemplate").
		Validates(firm.RuleMap{
			"URL":              {https},
			"DownloadTemplate": {firm.Backed()},
		}))
	firm.MustRegisterType(firm.NewDefinition[DownloadTemplate]().Validates(firm.RuleMap{
		"X64URL":   {https},
		"Arm64URL": {https},
	}))
}
