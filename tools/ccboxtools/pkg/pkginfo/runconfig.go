package pkginfo

import (
	"fmt"
	"os"
	"strconv"

	"github.com/s12chung/firm"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/jsonutil"
)

// RunConfigEnvVar is the container env var carrying the run's RunConfig JSON.
const RunConfigEnvVar = "CCBOX_CONFIG"

// RunConfig is the run's config asks the container acts on; the cli's name is CLI_PKGINFO's
type RunConfig struct {
	// CLIVersion is "latest" to track each release, or an exact version to stay on
	CLIVersion string `json:"cli_version"`
	// PortBindingsTCP are the default variant's port bindings; nil binds none
	PortBindingsTCP []string `json:"port_bindings_tcp,omitempty"`
	// HostGatewayTCP are the host gateway's ports; nil names none
	HostGatewayTCP []string `json:"host_gateway_tcp,omitempty"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[RunConfig]().
		Validates(firm.RuleMap{
			"CLIVersion": {VersionRule},
		}))
}

// RunConfigFromEnv parses the run's config off the env — a boot without one fails.
func RunConfigFromEnv() (RunConfig, error) {
	body := os.Getenv(RunConfigEnvVar)
	if body == "" {
		return RunConfig{}, fmt.Errorf("%s is unset", RunConfigEnvVar)
	}
	return RunConfigFromJSON(body)
}

// RunConfigFromJSON parses RunConfigEnvVar JSON, rejecting unknown fields and
// invalid values.
func RunConfigFromJSON(body string) (RunConfig, error) {
	config, err := jsonutil.Decode[RunConfig](body)
	return config, errs.Prefix("pkginfo", err)
}

// BoundPorts are the port bindings as port numbers.
func (c RunConfig) BoundPorts() ([]uint16, error) { return ports(c.PortBindingsTCP) }

// GatewayPorts are the host gateway ports as port numbers.
func (c RunConfig) GatewayPorts() ([]uint16, error) { return ports(c.HostGatewayTCP) }

// ports parses port strings — nil names none.
func ports(strs []string) ([]uint16, error) {
	if len(strs) == 0 {
		return nil, nil
	}
	ports := make([]uint16, 0, len(strs))
	for _, str := range strs {
		port, err := strconv.ParseUint(str, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("pkginfo: parse port %q: %w", str, err)
		}
		ports = append(ports, uint16(port))
	}
	return ports, nil
}
