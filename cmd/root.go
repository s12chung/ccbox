// Package cmd holds the ccbox cobra commands. It is thin: each command gathers
// config and calls one pkg/docker operation.
package cmd

import (
	"embed"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/util/flagptr"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
)

// Injected from main (package main can't be imported, so the embed FS comes in here).
var embedBuildContext embed.FS // Dockerfile + docker/* — the build context

// Shared flags.
var (
	flagTag string
	flagCLI *string
)

// loaded once together before any command runs
var (
	projectConfig *projectcfg.Config // layered config (user < project < local < flags)
	tagByProject  bool               // whether the project carries its own mise config
)

var rootCmd = &cobra.Command{
	Use:           "ccbox",
	Short:         "Run your harness in a hardened Docker devbox",
	Long:          "Run your harness in a hardened Docker devbox; `ccbox run [command...]` execs a command in it instead.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: func(_ *cobra.Command, args []string) error {
		switch {
		case len(args) > 1:
			return errors.New("accepts at most one session name")
		case len(args) == 1 && !runMode.Resume:
			return errors.New("a session name requires -r/--resume")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := runMode
		mode.Args = args
		return run(cmd.Context(), mode)
	},
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		for c := cmd; c != nil; c = c.Parent() {
			if c.Name() == doctorCmd.Name() {
				return nil // doctor loads its own way (doctor clis): no seeding, no config load
			}
		}

		provider.Load() // before cli.Load: AllowDomains's firm rule and expansion depends on loaded providers
		cli.Load()      // before rootSeed's picker and projectcfg.Load, which read cli.Names()
		if cmd.CalledAs() == cobra.ShellCompRequestCmd || cmd.CalledAs() == cobra.ShellCompNoDescRequestCmd {
			return nil
		}

		if err := rootSeed(); err != nil {
			return err
		}
		var err error
		projectConfig, err = projectcfg.Load(mustGetwd(), projectcfg.Config{CLIName: flagCLI}, runVNC)
		if err != nil {
			return err
		}
		tagByProject = osutil.Present(mise.ProjectConfigPath(projectConfig.ProjectDir()))
		return prompt.ProjectDirSize(projectConfig.ProjectDir())
	},
}

// Execute runs the CLI
func Execute(buildContext embed.FS) error {
	embedBuildContext = buildContext
	initMiseImage()
	return rootCmd.Execute()
}

func init() {
	// see run.go's init for more flags set for rootCmd
	f := rootCmd.Flags()
	f.BoolVarP(&runMode.Continue, "continue", "c", false, "continue the last session")
	f.BoolVarP(&runMode.Resume, "resume", "r", false, "resume a session: `ccbox -r <name>`, or bare for the picker")
	rootCmd.MarkFlagsMutuallyExclusive("continue", "resume")

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagTag, "tag", docker.DefaultTag, "devbox image tag")
	pf.Var(flagptr.String(&flagCLI), "cli", "override the coding CLI set in .ccbox.yaml")
	// unreachable error: "cli" is registered above
	must.Do(rootCmd.RegisterFlagCompletionFunc("cli", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		// a closure, so that cli.Load() runs first in PersistentPreRunE
		return cli.Names(), cobra.ShellCompDirectiveNoFileComp
	}))

	rootCmd.AddCommand(runCmd, buildCmd, pkginfoCmd, proxyCmd, reseedCmd, cleanCmd, configCmd, doctorCmd)
}

// rootSeed seeds the user-level harness state if missing
func rootSeed() error {
	if err := safeSeed(clitmpl.UserSeedFS(), clitmpl.UserDir(), false); err != nil {
		return err
	}
	if err := safeSeed(provider.UserSeedFS(), provider.UserDir(), false); err != nil {
		return err
	}
	if err := share.SafeSeedAgentsMd(); err != nil {
		return err
	}
	if err := mise.SeedConfig(mise.UserConfigPath()); err != nil {
		return err
	}
	return safeSeedUserConfig()
}

// safeSeedUserConfig seeds the user-level config template if missing
func safeSeedUserConfig() error {
	path := projectcfg.UserConfigFile()
	if osutil.Present(path) {
		return nil // the file already existed: no seed, so no prompt, no log
	}
	cliName, err := prompt.HarnessCLI(cli.Names())
	if err != nil {
		return err
	}
	return projectcfg.SeedConfig(path, cliName)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
