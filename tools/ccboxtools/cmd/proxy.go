package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/entrypoint"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

// The proxy subcommands run inside the container, re-exec'd by the boot and by exec
// sessions — users don't type them.
var proxyCmd = &cobra.Command{
	Use:    "proxy",
	Short:  "The egress wall's maintenance entries",
	Hidden: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := rootCmd.PersistentPreRunE(cmd, args); err != nil {
			return err
		}
		if err := identity.VerifyAndResetDrop(); err != nil {
			return err
		}
		return proxy.RequireNetNS()
	},
}

var proxyChildCmd = &cobra.Command{
	Use:   "child",
	Short: "Prove the drop and the egress wall inside the child netns, then run the boot's main work — the boot's one-shot proxy child",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return entrypoint.RunChild(cmd.Context())
	},
}

var proxyExecCmd = cmdWithExec(cmdWithExecParams{
	Use:   "exec [--] [command...]",
	Short: "Drop to ccbox inside the child netns and exec the command — the egress wall session's nsenter lands here",
	Run: func(_ context.Context, args []string) error {
		return entrypoint.ExecArgv(args)
	},
})

var proxyDesktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Exec the XFCE session on the bare server's display — the desktop session's landing inside the egress wall",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return entrypoint.ExecDesktop(cmd.Context(), vncInfo)
	},
}

func init() { proxyCmd.AddCommand(proxyChildCmd, proxyExecCmd, proxyDesktopCmd) }
