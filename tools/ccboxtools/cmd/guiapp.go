package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// guiAppKey keys the GUI app the guiapp parent checks into the command's context
type guiAppKey struct{}

var guiappCmd = &cobra.Command{
	Use:   "guiapp",
	Short: "Launch the GUI app the run's VNC_CONFIG names",
	// The children just trust the app this checks in — none runs without it
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		vnc = guiapp.Load() // this nearer hook replaces root's, so it loads vnc too
		if vnc == nil || vnc.GUIApp == nil {
			return fmt.Errorf("guiapp: %s names no gui_app", pkginfo.VNCConfigEnvVar)
		}
		cmd.SetContext(context.WithValue(cmd.Context(), guiAppKey{}, *vnc.GUIApp))
		return nil
	},
}

// guiAppFrom returns the GUI app the guiapp parent checked in — a child's invariant
func guiAppFrom(cmd *cobra.Command) pkginfo.GUIPkgInfo {
	app, ok := cmd.Context().Value(guiAppKey{}).(pkginfo.GUIPkgInfo)
	if !ok {
		panic("guiAppFrom: no gui app in context — the guiapp parent must check it in")
	}
	return app
}

var guiappExecCmd = &cobra.Command{
	Use:                   "exec [--] [args...]",
	Short:                 "Exec the GUI app now, erroring when it is not installed",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "--" { // cobra's arg split leaves the separator in
			args = args[1:]
		}
		return guiapp.Exec(guiAppFrom(cmd), args)
	},
}

var guiappAutostartCmd = &cobra.Command{
	Use:   "autostart",
	Short: "Wait out the boot's install, then exec — the session's XDG autostart entry",
	RunE:  func(cmd *cobra.Command, _ []string) error { return guiapp.Autostart(guiAppFrom(cmd)) },
}

func init() { guiappCmd.AddCommand(guiappExecCmd, guiappAutostartCmd) }
