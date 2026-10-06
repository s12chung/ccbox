// Package cmd holds the ccbox cobra commands. It is thin: each command gathers
// config and calls one pkg/docker operation.
package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/flagutils"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// Injected from main (package main can't be imported, so the embed FS comes in here).
var buildContext embed.FS // Dockerfile + docker/* — the build context

// Shared flags.
var (
	flagTag string
	flagCLI *string
)

// exitCode lets `run` propagate the container's exit status out through Execute.
var exitCode int

// projectConfig is the layered config (user < project < local < flags), loaded once
// before any command runs.
var projectConfig *projectcfg.Config

var rootCmd = &cobra.Command{
	Use:           "ccbox",
	Short:         "Hardened Docker devbox for Claude Code",
	Long:          "Hardened Docker devbox for Claude Code. With no subcommand, runs the devbox container interactively behind the egress wall.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          resumeArgs,
	RunE:          run,
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
		return err
	},
}

// Execute runs the CLI and returns the process exit code.
func Execute(build embed.FS) int {
	buildContext = build
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("command failed: %v", err)
		return 1
	}
	return exitCode
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagTag, "tag", docker.DefaultTag, "devbox image tag")
	pf.Var(flagutils.StringPtr(&flagCLI), "cli", "override the coding CLI set in .ccbox.yaml")
	// unreachable error: "cli" is registered above
	must.Do(rootCmd.RegisterFlagCompletionFunc("cli", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		// a closure, so that cli.Load() runs first in PersistentPreRunE
		return cli.Names(), cobra.ShellCompDirectiveNoFileComp
	}))

	rootCmd.AddCommand(buildCmd, pkginfoCmd, proxyCmd, reseedCmd, cleanCmd, configCmd, doctorCmd)
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
	return safeSeedUserConfig()
}

// safeSeedUserConfig seeds the user-level config template if missing
func safeSeedUserConfig() error {
	if ioutil.Present(projectcfg.UserConfigFile()) {
		return nil // the file already existed: no seed, so no prompt, no log
	}
	cli, err := pick.Select(
		"Select a harness CLI",
		[]string{
			fmt.Sprintf("(stored in %s)", userdir.Tilde(projectcfg.UserConfigFile())),
			fmt.Sprintf("see %s to plug your own", userdir.Tilde(filepath.Join(clitmpl.UserDir(), "README.md"))),
		},
		cli.Names())
	if err != nil {
		return err
	}
	if cli == "" { // no terminal to show the picker
		return fmt.Errorf("no harness CLI selected: rerun in a terminal to pick one, or set cli in %s", projectcfg.UserConfigFile())
	}
	seeded, err := projectcfg.SeedUserConfig(cli)
	if err != nil || seeded == "" { // "" = lost a seed race: no seed, so no log
		return err
	}
	log.Infof("seeded %s", seeded)
	return nil
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
