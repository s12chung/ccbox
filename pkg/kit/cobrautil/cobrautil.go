// Package cobrautil contains cobra helpers
package cobrautil

import "github.com/spf13/cobra"

// WithinCmd reports whether cmd is c or any of its subcommands
func WithinCmd(cmd, c *cobra.Command) bool {
	for cur := cmd; cur != nil; cur = cur.Parent() {
		if cur == c {
			return true
		}
	}
	return false
}
