// Package cmd holds the ccboxtools cobra commands.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
)

// vnc is the run's parsed VNC session, loaded in root's PersistentPreRunE — nil means no vnc
var vnc *pkginfo.VNCInfo

var rootCmd = &cobra.Command{
	Use:           "ccboxtools",
	Short:         "Tools to maintain ccbox's container",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(*cobra.Command, []string) error {
		vnc = guiapp.Load()
		return nil
	},
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("command failed: %v", err)
		return 1
	}
	return 0
}

func init() { rootCmd.AddCommand(updateCmd, entrypointCmd, guiappCmd) }
