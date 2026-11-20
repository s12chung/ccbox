package cobrakit

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestWithinCmd(t *testing.T) {
	sub, subSub := &cobra.Command{Use: "sub"}, &cobra.Command{Use: "subsub"}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(sub)
	sub.AddCommand(subSub)

	strangerSub := &cobra.Command{Use: "strangersub"}
	stranger := &cobra.Command{Use: "stranger"}
	stranger.AddCommand(strangerSub)

	tests := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{name: "the command itself", cmd: root, want: true},
		{name: "a subcommand", cmd: sub, want: true},
		{name: "a subcommand's subcommand", cmd: subSub, want: true},
		{name: "an unrelated command", cmd: stranger, want: false},
		{name: "an unrelated command's subcommand", cmd: strangerSub, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, WithinCmd(tt.cmd, root))
		})
	}
}
