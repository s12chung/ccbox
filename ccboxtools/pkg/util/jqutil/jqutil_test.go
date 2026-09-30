package jqutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectOne_Errors(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		doc      any
		wantErr  string
	}{
		{
			"parse error",
			".version]",
			map[string]any{"version": "3.14.4"},
			`selector ".version]":`,
		},
		{
			"compile error",
			"missingfn",
			map[string]any{"version": "3.14.4"},
			`selector "missingfn": function not defined: missingfn/0`,
		},
		{
			"no result",
			".files[].url",
			map[string]any{"files": []any{}},
			`selector ".files[].url": no result`,
		},
		{
			"multi-result",
			".files[].url",
			map[string]any{"files": []any{map[string]any{"url": "a.deb"}, map[string]any{"url": "b.rpm"}}},
			`selector ".files[].url": more than one result`,
		},
		{
			"runtime error",
			".a",
			"a string",
			`selector ".a": expected an object but got: string`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := selectOne(tt.selector, tt.doc)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSelectStr(t *testing.T) {
	s, err := SelectStr(".version", map[string]any{"version": "3.14.4"})
	require.NoError(t, err)
	assert.Equal(t, "3.14.4", s)
}

func TestSelectStr_Errors(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		doc      any
		wantErr  string
	}{
		{
			"non-string result",
			".count",
			map[string]any{"count": 3},
			`selector ".count": result is not a string:`,
		},
		{
			"no result",
			".files[].url",
			map[string]any{"files": []any{}},
			`selector ".files[].url": no result`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SelectStr(tt.selector, tt.doc)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// selectDoc is SelectFields' test select struct: hidden carries an unexported
// field, which SelectFields must skip.
type selectDoc struct {
	Version string
	URL     string
	hidden  string
}

func TestSelectFields(t *testing.T) {
	doc := map[string]any{"version": "3.14.4", "url": "https://x/a.deb"}

	sel, err := SelectFields(selectDoc{Version: ".version", URL: ".url", hidden: "keep"}, doc)
	require.NoError(t, err)
	assert.Equal(t, selectDoc{Version: "3.14.4", URL: "https://x/a.deb", hidden: "keep"}, sel)
}

func TestSelectFields_Errors(t *testing.T) {
	doc := map[string]any{"version": "3.14.4"}

	t.Run("selector error", func(t *testing.T) {
		_, err := SelectFields(selectDoc{Version: ".files[].url"}, map[string]any{"files": []any{}})
		require.Error(t, err)
		assert.ErrorContains(t, err, `selector ".files[].url": no result`)
	})

	t.Run("non-string field panics", func(t *testing.T) {
		type badSelectDoc struct {
			Version string
			Count   int
		}
		assert.PanicsWithValue(t, "SelectFields: Count is not a string field", func() {
			_, _ = SelectFields(badSelectDoc{Version: ".version"}, doc)
		})
	})
}

func TestSelectOne(t *testing.T) {
	res, err := selectOne(".version", map[string]any{"version": "3.14.4"})
	require.NoError(t, err)
	assert.Equal(t, "3.14.4", res)
}
