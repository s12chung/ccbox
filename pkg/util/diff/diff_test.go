package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLines(t *testing.T) {
	tests := []struct {
		name     string
		oldLines []string
		newLines []string
		want     string
	}{
		{
			name:     "NoChange",
			oldLines: []string{"a", "b"},
			newLines: []string{"a", "b"},
			want:     "",
		},
		{
			name:     "EmptyBoth",
			oldLines: nil,
			newLines: nil,
			want:     "",
		},
		{
			name:     "AllAdded",
			oldLines: nil,
			newLines: []string{"a", "b"},
			want:     "+a\n+b",
		},
		{
			name:     "AllRemoved",
			oldLines: []string{"a", "b"},
			newLines: nil,
			want:     "-a\n-b",
		},
		{
			name:     "MixedAroundShared",
			oldLines: []string{"a", "b", "c"},
			newLines: []string{"a", "x", "c"},
			want:     "-b\n+x",
		},
		{
			name:     "RemovalsLeadAdditions",
			oldLines: []string{"a", "b"},
			newLines: []string{"b", "c"},
			want:     "-a\n+c",
		},
		{
			name:     "Reordered",
			oldLines: []string{"a", "b"},
			newLines: []string{"b", "a"},
			want:     "-a\n+a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Lines(tt.oldLines, tt.newLines))
		})
	}
}
