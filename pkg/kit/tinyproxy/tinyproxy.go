// Package tinyproxy provides the egress wall's configs
package tinyproxy

import "embed"

// Config is the egress wall's configs, rooted at its content.
//
//go:embed tinyproxy.conf
var Config embed.FS
