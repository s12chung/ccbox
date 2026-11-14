package cfg

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/models/provider"
	"github.com/s12chung/ccbox/pkg/models/runtime"
)

func TestAllowlistDefaults_ExcludeProviders(t *testing.T) {
	// the defaults are tooling only: providers live under their own aliases —
	// ccbox-all-providers resolves every one of them
	providerDomains, ok := provider.DomainsFor(provider.AllProvidersAlias)
	require.True(t, ok)
	for _, d := range providerDomains {
		assert.NotContainsf(t, AllowlistDefaults(), d, "provider domain %s", d)
	}
}

func TestAllowlistDefaults_ExcludeRuntimes(t *testing.T) {
	// the defaults are tooling only: the runtimes live under their own aliases —
	// ccbox-all-runtimes resolves every one of them
	runtimeDomains, ok := runtime.DomainsFor(runtime.AllRuntimesAlias)
	require.True(t, ok)
	for _, d := range runtimeDomains {
		assert.NotContainsf(t, AllowlistDefaults(), d, "runtime domain %s", d)
	}
}

func TestAllowlistDefaults_ExcludeGuiApp(t *testing.T) {
	// the GUI app's domains enter only through a vnc load's ccbox-set-harness expansion —
	// a headless run never downloads the app
	for _, d := range guiapp.App.Setup.Proxied.AllowDomains {
		assert.NotContainsf(t, AllowlistDefaults(), d, "guiapp: %s", d)
	}
}

func TestAllowlistDefaults_DomainsValid(t *testing.T) {
	for _, d := range AllowlistDefaults() {
		assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s", d)
	}
}
