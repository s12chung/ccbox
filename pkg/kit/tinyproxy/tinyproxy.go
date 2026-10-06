// Package tinyproxy it's configs
package tinyproxy

import (
	"embed"
	"io/fs"

	"github.com/s12chung/ccbox/pkg/util/must"
)

// ConfFile is tinyproxy.conf's name within config
const ConfFile = "tinyproxy.conf"

// config is tinyproxy's configs, rooted at its content.
//
//go:embed tinyproxy.conf
var config embed.FS

// MustConf returns the tinyproxy.conf, panicking on the read error — unreachable for
// an embed.
func MustConf() []byte { return must.Get(fs.ReadFile(config, ConfFile)) }
