package pkginfo

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGUIPkgInfo_Validate(t *testing.T) {
	tests := []struct {
		name string
		g    GUIPkgInfo
		want string
	}{
		{"ok", GUIPkgInfo{PkgInfo: PkgInfo{Name: "app", Npm: &Npm{Package: "a"}}}, ""},
		{"no install source", GUIPkgInfo{Args: []string{"--no-sandbox"}}, "OneNotNil"},
		{"path escape name", GUIPkgInfo{PkgInfo: PkgInfo{Name: "../evil", Npm: &Npm{Package: "a"}}}, "Name.Match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(tt.g)
			if tt.want == "" {
				assert.Empty(t, errMap)
				return
			}
			require.NotEmpty(t, errMap)
			assert.Contains(t, errMap.Error(), tt.want)
		})
	}
}
