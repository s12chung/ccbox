package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/entrypoint"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
)

// The egress wall's maintenance entries — the boot re-execs this binary into
// them, never typed by a user. Their shared landing: drop to ccbox and prove
// the child netns, fail closed, before any of them runs.
var proxyCmd = &cobra.Command{
	Use:    "proxy",
	Short:  "The egress wall's maintenance entries",
	Hidden: true,
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		if err := identity.Drop(); err != nil {
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

func init() {
	proxyCmd.AddCommand(proxyChildCmd)
	proxyCmd.AddCommand(proxyExecCmd)
}
