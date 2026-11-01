// Package cmd holds the ccboxtools cobra commands.
package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
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
func Execute(ctx context.Context) int {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Errorf("command failed: %v", err)
		return 1
	}
	return 0
}

// cmdWithExecParams are cmdWithExec's params.
type cmdWithExecParams struct {
	Use   string
	Short string
	Run   func(ctx context.Context, args []string) error
}

// cmdWithExec builds a command that runs the session's argv.
func cmdWithExec(params cmdWithExecParams) *cobra.Command {
	return &cobra.Command{
		Use:                   params.Use,
		Short:                 params.Short,
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		Args:                  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "--" { // cobra's arg split leaves the separator in
				args = args[1:]
			}
			return params.Run(cmd.Context(), args)
		},
	}
}

func init() { rootCmd.AddCommand(updateCmd, entrypointCmd, proxyCmd, execCmd, guiappCmd, allowlistCmd) }
