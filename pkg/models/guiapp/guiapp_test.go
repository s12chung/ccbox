package guiapp

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup(t *testing.T) {
	app, err := lookup(App.Name)
	require.NoError(t, err)
	assert.Equal(t, App, app)

	assert.Contains(t, Names(), App.Name)

	_, err = lookup("emacs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not one of [zcode]")
}

func TestMustFor(t *testing.T) {
	assert.Equal(t, App, *MustFor(App.Name))
	assert.PanicsWithError(t, `guiapp: "" is not one of [zcode]`, func() { MustFor("") })
	assert.PanicsWithError(t, `guiapp: "emacs" is not one of [zcode]`, func() { MustFor("emacs") })
}

// App is a declaration — the firm rules are its spec; this is its check
func TestApp_Valid(t *testing.T) {
	assert.Nil(t, firm.ValidateAny(App).ToNil())
}
