// Package docker drives the devbox lifecycle (build/run/proxy) over the Docker
// Engine SDK, replacing the former Makefile recipes.
package docker

// Shared across commands: the default image tag and the wall's fixed names.
// Constants used by only one command live in that command's file.
const (
	DefaultTag = "s12chung/ccbox:latest"

	networkName = "ccbox-wall"   // internal network with no direct egress
	egressName  = "ccbox-egress" // tinyproxy container, the only way out
)
