package app

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRules(t *testing.T) {
	good := Setup{
		Env:       map[string]string{"MYCLI_SKIP_UPDATE": "1"},
		DataBinds: map[string]*string{".mycli/store": nil},
		Proxied: Proxied{
			AllowDomains: []string{"ccbox-all-providers", "example.dev"},
			DataBinds:    map[string]*string{".mycli/proxy-store": nil},
		},
	}

	assert.Nil(t, firm.ValidateAny(good).ToNil())

	tests := []struct {
		name  string
		setup Setup
		err   string
	}{
		{"env key is not an env var", Setup{Env: map[string]string{"not-env": "1"}}, "Env"},
		{"env value is empty", Setup{Env: map[string]string{"MYCLI_SKIP_UPDATE": ""}}, "Env"},
		{"data bind escapes the home dir", Setup{DataBinds: map[string]*string{"../escape": nil}}, "DataBinds"},
		{"allow domain is neither a domain nor an alias", Setup{Proxied: Proxied{AllowDomains: []string{"ccbox-unknown"}}}, "AllowDomains"},
		{"proxied data bind escapes the home dir", Setup{Proxied: Proxied{DataBinds: map[string]*string{"../escape": nil}}}, "DataBinds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(tt.setup)
			require.NotNil(t, errMap)
			assert.Contains(t, errMap.Error(), tt.err)
		})
	}
}

func TestSetup_DataBindSeedMap(t *testing.T) {
	s := Setup{
		DataBinds: map[string]*string{
			".zcode/v2":                   nil,        // bound dir
			".zcode/v2/credential-secret": new("sec"), // carried by the bound dir
			".other/auth.json":            new("{}"),  // no bound ancestor
		},
		Proxied: Proxied{DataBinds: map[string]*string{".zcode/v2/setting.json": new("{}")}},
	}

	assert.Equal(t, map[string]*string{
		".zcode-v2":                   nil,
		".zcode-v2/credential-secret": new("sec"),
		".other-auth.json":            new("{}"),
	}, s.DataBindSeedMap(true), "the plain refs alone")
	assert.Equal(t, map[string]*string{
		".zcode-v2":                   nil,
		".zcode-v2/credential-secret": new("sec"),
		".other-auth.json":            new("{}"),
		".zcode-v2/setting.json":      new("{}"),
	}, s.DataBindSeedMap(false), "the proxied seed nests under the bound dir's ref")
	assert.Empty(t, Setup{}.DataBindSeedMap(false))

	nested := Setup{DataBinds: map[string]*string{
		".mycli":            nil,
		".mycli/sub/secret": new("s"),
	}}
	assert.Equal(t, map[string]*string{
		".mycli":            nil,
		".mycli/sub/secret": new("s"),
	}, nested.DataBindSeedMap(true), "the rest of the path stays real — the bind carries it 1:1")

	deep := Setup{DataBinds: map[string]*string{
		".local/share/opencode/auth.json":     new("a"), // no bound ancestor
		".local/share/opencode/sessions.json": new("s"), // another file in the same tree
	}}
	assert.Equal(t, map[string]*string{
		".local-share-opencode-auth.json":     new("a"),
		".local-share-opencode-sessions.json": new("s"),
	}, deep.DataBindSeedMap(true), "deep files with no bound ancestor: each path slugs flat, distinctly")
}

func TestSetup_DataBindMounts(t *testing.T) {
	s := Setup{
		DataBinds: map[string]*string{
			".mycli":                       nil,       // bound dir
			".mycli/auth.json":             new("{}"), // carried by the bound dir
			".other/store.json":            new("{}"), // no bound ancestor: binds as a file
			".other/nested":                nil,       // dir despite the extension
			".mycli-auth":                  new("{}"), // home-root file: no dir to be carried by
			".other/nested/deep/auth.json": new("{}"), // carried by the bound dir above
			".other/deep/a.json":           new("{}"), // deep files with no bound ancestor:
			".other/deep/b.json":           new("{}"), // mount as files, distinctly
		},
		Proxied: Proxied{DataBinds: map[string]*string{".mycli/proxy-store.json": new("{}")}},
	}

	assert.Equal(t, map[string]string{
		".mycli":             ".mycli",
		".other-store.json":  ".other/store.json",
		".other-nested":      ".other/nested",
		".mycli--auth":       ".mycli-auth",
		".other-deep-a.json": ".other/deep/a.json",
		".other-deep-b.json": ".other/deep/b.json",
	}, s.DataBindMounts(true))
	assert.Equal(t, s.DataBindMounts(true), s.DataBindMounts(false),
		"the proxied seed nests under the bound dir, carried like the rest")
}

func TestSetup_DataBindContainerDirs(t *testing.T) {
	s := Setup{
		DataBinds: map[string]*string{
			".mycli":               nil,       // bound dir: itself is the mountpoint
			".mycli/sub/secret":    new("{}"), // carried file: adds its parent
			".other/store.json":    new("{}"), // standalone file: adds its parent
			".other/nested":        nil,       // dir despite the extension: itself is the mountpoint
			".other/deep/a.json":   new("{}"), // standalone file: adds its parent
			".other/deep/b/c.json": new("{}"), // deeper standalone file: adds its parent
			".mycli-auth":          new("{}"), // home-root file: adds none — home is its parent
		},
		Proxied: Proxied{DataBinds: map[string]*string{".other/proxy/deep.json": new("{}")}},
	}

	assert.ElementsMatch(t, []string{
		".mycli",
		".mycli/sub",
		".other",
		".other/nested",
		".other/deep",
		".other/deep/b",
	}, s.DataBindContainerDirs(true), "home-relative: dir binds and file bind parents")

	assert.ElementsMatch(t, []string{
		".mycli",
		".mycli/sub",
		".other",
		".other/nested",
		".other/deep",
		".other/deep/b",
		".other/proxy", // the proxied overlay
	}, s.DataBindContainerDirs(false))
}
