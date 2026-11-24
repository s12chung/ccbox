// Package cmd holds the ccbox cobra commands. It is thin: each command gathers
// config and calls one pkg/docker operation.
package cmd

import (
	"context"
	"embed"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/dockerfile"
	"github.com/s12chung/ccbox/pkg/kit/cobrakit"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/provider"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/util/flagptr"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
)

// embedBuildContext is injected from main — package main can't be imported; it holds
// the Dockerfile and docker/*, the build context
var embedBuildContext embed.FS

var flags struct {
	CLI             *string
	PortBindingsTCP *string
	HostGatewayTCP  *string
	VNC             bool
	NoProxy         bool
}

// loaded once together before any command runs
var (
	config       *cfg.Config          // layered config (user < project < local < flags)
	miseRuntimes runtime.MiseRuntimes // the picked mise config's runtime selection
	tagByProject bool                 // whether the project carries its own mise config
)

var rootCmd = &cobra.Command{
	Use:           "ccbox",
	Short:         "Run your harness in a hardened Docker devbox",
	Long:          "Run your harness in a hardened Docker devbox; `ccbox exec [command...]` execs a command in it instead.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return execSession(cmd.Context(), execMode)
	},
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if cobrakit.WithinCmd(cmd, doctorCmd) {
			return nil // doctor loads its own way (doctor clis): no seeding, no config load
		}

		provider.Load() // before cli.Load: AllowDomains's firm rule and expansion depends on loaded providers
		runtime.Load()  // before rootSeed's picker and cfg.Load, which read runtime.All() and the aliases
		cli.Load()      // before rootSeed's picker and cfg.Load, which read cli.Names()
		if cmd.CalledAs() == cobra.ShellCompRequestCmd || cmd.CalledAs() == cobra.ShellCompNoDescRequestCmd {
			return nil
		}

		if err := rootSeed(cmd.Context()); err != nil {
			return err
		}
		var err error
		config, err = cfg.Load(mustGetwd(), cfg.Flags{
			CLI:             flags.CLI,
			VNC:             flags.VNC,
			PortBindingsTCP: flags.PortBindingsTCP,
			HostGatewayTCP:  flags.HostGatewayTCP,
			NoProxy:         flags.NoProxy || cmd == runCmd,
			ProxyCmd:        cobrakit.WithinCmd(cmd, proxyCmd),
		})
		if err != nil {
			return err
		}
		miseRuntimes, err = runtime.FromMiseConfig(config.ProjectDir())
		if err != nil {
			return err
		}
		tagByProject = osutil.Present(mise.ProjectConfigPath(config.ProjectDir()))
		return prompt.ProjectDirSize(config.ProjectDir())
	},
}

// Execute runs the CLI
func Execute(buildContext embed.FS) error {
	embedBuildContext = buildContext
	return rootCmd.Execute()
}

func init() {
	// see exec.go's init for more flags set for rootCmd
	pf := rootCmd.PersistentFlags()
	pf.Var(flagptr.String(&flags.CLI), "cli", "override the coding CLI set in .ccbox.yaml")
	pf.Var(flagptr.String(&flags.PortBindingsTCP), "port-bindings-tcp", "bind tcp ports on localhost, host = container port")
	pf.Var(flagptr.String(&flags.HostGatewayTCP), "host-gateway-tcp", "open tcp ports on the host to the devbox at host.docker.internal")
	// unreachable error: "cli" is registered above
	must.Do(rootCmd.RegisterFlagCompletionFunc("cli", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		// a closure, so that cli.Load() runs first in PersistentPreRunE
		return cli.Names(), cobra.ShellCompDirectiveNoFileComp
	}))

	rootCmd.AddCommand(runCmd, execCmd, buildCmd, pkginfoCmd, proxyCmd, cleanCmd, psCmd, configCmd, doctorCmd, miseCmd)
}

// rootSeed seeds the user-level harness state if missing
func rootSeed(ctx context.Context) error {
	if err := share.SafeSeedAgentsMd(); err != nil {
		return err
	}
	if err := share.SafeSeedSkills(); err != nil {
		return err
	}
	if err := safeSeedUserConfig(ctx); err != nil {
		return err
	}
	_, err := seedMiseConfig(ctx, mise.UserConfigPath())
	return err
}

// safeSeedUserConfig seeds the user-level config template if missing
func safeSeedUserConfig(ctx context.Context) error {
	path := cfg.UserConfigFile()
	if osutil.Present(path) {
		return nil // the file already existed: no seed, so no prompt, no log
	}
	cliName, err := prompt.HarnessCLI(cli.All())
	if err != nil {
		return err
	}
	refs, err := dockerfile.LatestImages(ctx)
	if err != nil {
		return err
	}
	return cfg.SeedConfig(path, cliName, refs)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
