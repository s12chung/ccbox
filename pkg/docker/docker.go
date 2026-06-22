// Package docker drives the devbox lifecycle (build/run/proxy) over the Docker
// Engine SDK, replacing the former Makefile recipes.
package docker

import "github.com/docker/docker/client"

// Shared across commands: the default image tag and the wall's fixed names.
// Constants used by only one command live in that command's file.
const (
	DefaultTag = "s12chung/ccbox:latest"

	networkName = "ccbox-wall"   // internal network with no direct egress
	egressName  = "ccbox-egress" // tinyproxy container, the only way out
)

// Client wraps the Docker SDK client.
type Client struct {
	cli *client.Client
}

// New connects to the Docker daemon from the environment (DOCKER_HOST etc.),
// negotiating the API version with the server.
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}
