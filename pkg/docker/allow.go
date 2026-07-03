package docker

import (
	"fmt"
	"regexp"
	"strings"
)

// allowFileName is the tinyproxy filter file, generated and copied into tinyproxyDir.
const allowFileName = "allow.txt"

// AllowOverride builds the embedfs override that seeds the wall's allow.txt from the
// (already resolved) domain list.
func AllowOverride(domains []string) map[string][]byte {
	return map[string][]byte{allowFileName: renderAllow(domains)}
}

// renderAllow turns each bare domain into an ERE filter line ((^|\.)domain$),
// matching the apex and any subdomain.
func renderAllow(domains []string) []byte {
	var b strings.Builder
	for _, d := range domains {
		fmt.Fprintf(&b, "(^|\\.)%s$\n", regexp.QuoteMeta(d))
	}
	return []byte(b.String())
}
