package mise

import (
	"github.com/s12chung/ccbox/pkg/kit/miseutil"
	"github.com/s12chung/ccbox/pkg/kit/pypi"
)

// init registers PyPI's index as the pypi: tools' list source
func init() { miseutil.RegisterSource(pypi.Prefix, pypi.Index{}) }
