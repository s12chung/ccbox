package pkginfo

import (
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Npm installs from the npm registry's "latest" dist-tag.
type Npm struct {
	Package string `json:"package" yaml:"package"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Npm]().Validates(firm.RuleMap{
		// npm package name, scoped or not, e.g. @anthropic-ai/claude-code
		"Package": {rule.Match{Regexp: regexp.MustCompile(`^(@[a-z0-9-]+/)?[a-z0-9][a-z0-9._-]*$`)}},
	}))
}
