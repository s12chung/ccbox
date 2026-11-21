package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

func TestPrintDevboxStatuses(t *testing.T) {
	tests := []struct {
		name     string
		statuses []docker.DevboxStatus
		want     string
	}{
		{
			name:     "None",
			statuses: nil,
			want:     "no devboxes for this project — boot one with `ccbox proxy` or `ccbox exec`\n",
		},
		{
			name: "Rows",
			statuses: []docker.DevboxStatus{
				{Name: "ccbox-proj", State: "running", Execs: 2, Foreground: true},
				{Name: "ccbox-proj-plain", State: "exited"},
			},
			want: "NAME                       STATE    EXECS\n" +
				"ccbox-proj (foregrounded)  running  2\n" +
				"ccbox-proj-plain           exited   0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			restore := log.SwapInfoWriter(&out)
			defer restore()

			require.NoError(t, printDevboxStatuses(tt.statuses))
			assert.Equal(t, tt.want, out.String())
		})
	}
}
