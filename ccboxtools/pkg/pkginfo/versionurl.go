package pkginfo

import (
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// VersionURL installs from a URL whose body is a bare version — xAI's channel
// endpoints, e.g. https://x.ai/cli/stable -> 1.0.5. The download URL templates
// carry a literal $version, substituted at install.
type VersionURL struct {
	URL           string `json:"url"             yaml:"url"`
	LinuxX64URL   string `json:"linux_x64_url"   yaml:"linux_x64_url"`
	LinuxArm64URL string `json:"linux_arm64_url" yaml:"linux_arm64_url"`
}

func init() {
	// https endpoints; download templates may carry a literal $version
	https := rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	firm.MustRegisterType(firm.NewDefinition[VersionURL]().Validates(firm.RuleMap{
		"URL":           {https},
		"LinuxX64URL":   {https},
		"LinuxArm64URL": {https},
	}))
}
