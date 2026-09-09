// Package cmd holds the ccboxtools cobra commands.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
)

var rootCmd = &cobra.Command{
	Use:           "ccboxtools",
	Short:         "Tools to maintain ccbox's container",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("command failed: %v", err)
		return 1
	}
	return 0
}

func init() { rootCmd.AddCommand(updateCmd, entrypointCmd) }
