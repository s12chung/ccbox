package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/guiapp"
)

var guiappCmd = &cobra.Command{
	Use:   "guiapp",
	Short: "Launch the GUI app the run's GUIAPP_PKGINFO names",
}

var guiappExecCmd = &cobra.Command{
	Use:                   "exec [--] [args...]",
	Short:                 "Exec the GUI app now, erroring when it is not installed",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ArbitraryArgs,
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "--" { // cobra's arg split leaves the separator in
			args = args[1:]
		}
		return guiapp.Exec(args)
	},
}

var guiappAutostartCmd = &cobra.Command{
	Use:   "autostart",
	Short: "Wait out the boot's install, then exec — the session's XDG autostart entry",
	RunE:  func(*cobra.Command, []string) error { return guiapp.Autostart() },
}

func init() { guiappCmd.AddCommand(guiappExecCmd, guiappAutostartCmd) }
