// Package cmd holds the ccbox cobra commands. It is thin: each command gathers
// config and calls one pkg/docker operation.
package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/flagutils"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/log"
)

// Injected from main (package main can't be imported, so the embed FSes come in here).
var (
	buildContext embed.FS // Dockerfile + docker/image/* — the build context
	proxyConfig  embed.FS // docker/tinyproxy/* — the egress wall configs
)

// Shared flags.
var (
	flagTag string
	flagCLI *string
)

// exitCode lets `run` propagate the container's exit status out through Execute.
var exitCode int

// projectCfg is the layered config (user < project < local < flags), loaded once
// before any command runs.
var projectCfg *projectcfg.Config

var rootCmd = &cobra.Command{
	Use:           "ccbox",
	Short:         "Hardened Docker devbox for Claude Code",
	Long:          "Hardened Docker devbox for Claude Code. With no subcommand, runs the devbox container interactively behind the egress wall.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          resumeArgs,
	RunE:          runDevbox,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if cmd.CalledAs() == cobra.ShellCompRequestCmd || cmd.CalledAs() == cobra.ShellCompNoDescRequestCmd {
			return nil // completion only reads flag/CLI definitions: no seeding, no config load
		}
		if err := safeSeedUserClis(); err != nil {
			return err
		}
		if err := safeSeedUserConfig(); err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectCfg, err = projectcfg.Load(cwd, projectcfg.Config{CLI: flagCLI})
		return err
	},
}

// Execute runs the CLI and returns the process exit code.
func Execute(build, proxy embed.FS) int {
	buildContext = build
	proxyConfig = proxy
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
	if err := rootCmd.RegisterFlagCompletionFunc("cli", cobra.FixedCompletions(harness.Names(), cobra.ShellCompDirectiveNoFileComp)); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(buildCmd, pkginfoCmd, proxyCmd, reseedCmd, cleanCmd, configCmd)
}

// safeSeedUserClis seeds harness.UserCLIsDir() if missing
func safeSeedUserClis() error {
	return safeSeed(fsutil.MustNewFS(harness.SeedUserClisFS()), harness.UserCLIsDir(), false)
}

// safeSeedUserConfig seeds the user-level config template if missing
func safeSeedUserConfig() error {
	if !ioutil.Missing(projectcfg.UserConfigFile()) {
		return nil // the file already existed: no seed, so no prompt, no log
	}
	cli, err := pick.Select(
		"Select a harness CLI",
		[]string{
			fmt.Sprintf("(stored in %s)", userdir.Tilde(projectcfg.UserConfigFile())),
			fmt.Sprintf("see %s to plug your own", userdir.Tilde(filepath.Join(harness.UserCLIsDir(), "README.md"))),
		},
		harness.Names())
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
