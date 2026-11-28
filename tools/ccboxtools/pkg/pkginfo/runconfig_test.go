package pkginfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunConfigFromJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want RunConfig
	}{
		{"latest", `{"cli_version":"latest"}`, RunConfig{CLIVersion: LatestVersion}},
		{
			"pinned with ports",
			`{"cli_version":"0.9.2","port_bindings_tcp":["3000","7200"],"host_gateway_tcp":["5432"]}`,
			RunConfig{CLIVersion: "0.9.2", PortBindingsTCP: []string{"3000", "7200"}, HostGatewayTCP: []string{"5432"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunConfigFromJSON(tt.body)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	invalid := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `{"cli_version":"latest","cli_name":"pi"}`, "unknown field"},
		{"bad json", `{`, "parse"},
		{"no version", `{}`, "CLIVersion.Or"},
		{"range version", `{"cli_version":"^1.0"}`, "CLIVersion.Or"},
		{"dist-tag version", `{"cli_version":"next"}`, "CLIVersion.Or"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RunConfigFromJSON(tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRunConfigFromEnv(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv(RunConfigEnvVar, `{"cli_version":"latest"}`)

		got, err := RunConfigFromEnv()

		require.NoError(t, err)
		assert.Equal(t, RunConfig{CLIVersion: LatestVersion}, got)
	})

	t.Run("unset fails the boot", func(t *testing.T) {
		t.Setenv(RunConfigEnvVar, "")

		_, err := RunConfigFromEnv()

		require.Error(t, err)
		assert.ErrorContains(t, err, RunConfigEnvVar)
	})
}

func TestRunConfig_Ports(t *testing.T) {
	// both asks parse the same way
	asks := []struct {
		name string
		ask  func(RunConfig) ([]uint16, error)
	}{{"bound", RunConfig.BoundPorts}, {"gateway", RunConfig.GatewayPorts}}
	tests := []struct {
		name    string
		ports   []string
		want    []uint16
		wantErr string
	}{
		{"nil", nil, nil, ""},
		{"one", []string{"3000"}, []uint16{3000}, ""},
		{"many", []string{"3000", "5900", "7200"}, []uint16{3000, 5900, 7200}, ""},
		{"garbage", []string{"3000", "http"}, nil, `parse port "http"`},
		{"overflows a port", []string{"65536"}, nil, `parse port "65536"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ask := range asks {
				t.Run(ask.name, func(t *testing.T) {
					ports, err := ask.ask(RunConfig{PortBindingsTCP: tt.ports, HostGatewayTCP: tt.ports})
					if tt.wantErr == "" {
						require.NoError(t, err)
						assert.Equal(t, tt.want, ports)
						return
					}
					require.ErrorContains(t, err, tt.wantErr)
				})
			}
		})
	}
}
