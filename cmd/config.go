package cmd

import (
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/printutil"
	"github.com/s12chung/ccbox/pkg/util/uslice"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the effective .ccbox.yaml with its aliases expanded",
	RunE: func(_ *cobra.Command, _ []string) error {
		log.Info("# Run `ccbox config defaults` for the alias expansions")
		printLoadedPaths()
		out, err := yaml.Marshal(projectConfig)
		if err != nil {
			return err
		}
		log.Info(strings.TrimRight(string(out), "\n"))
		printGuardMountWarnings()
		return nil
	},
}

// printLoadedPaths lists the config files loaded, in load order
func printLoadedPaths() {
	loaded := projectcfg.LoadedPaths(projectConfig.ProjectDir())
	if len(loaded) == 0 {
		return
	}
	log.Info("\n# Config files, in load order:")
	for _, path := range loaded {
		log.Infof("#   %s", userdir.Tilde(path))
	}
	log.Info("")
}

// printGuardMountWarnings prints what the guard mounts will do at run. Mask dirs and bind
// hosts are present-checked like any dir; globs can't be — their expansion shows what's armed instead.
func printGuardMountWarnings() {
	tmpfsMasks := projectConfig.TmpfsMasksAbsent()
	volumeMasks := projectConfig.VolumeMasksAbsent()
	globs := projectConfig.ReadOnlyGlobsExpanded()
	binds := projectConfig.ReadOnlyBindsAbsent()
	if len(tmpfsMasks) > 0 || len(volumeMasks) > 0 || len(globs) > 0 || len(binds) > 0 {
		log.Info("")
	}
	if len(tmpfsMasks) > 0 {
		log.Infof("# tmpfs_masks not in project, not masked: %s", strings.Join(tmpfsMasks, ", "))
	}
	if len(volumeMasks) > 0 {
		log.Infof("# volume_masks not in project, not masked: %s", strings.Join(volumeMasks, ", "))
	}
	if len(globs) > 0 {
		log.Infof("# read_only_globs matches re-mount read-only at run: %s", strings.Join(globs, ", "))
	}
	if len(binds) > 0 {
		log.Infof("# read_only_binds not on host, not mounted: %s", strings.Join(slices.Sorted(maps.Keys(binds)), ", "))
	}
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter .ccbox.yaml template to fill in",
	RunE: func(_ *cobra.Command, _ []string) error {
		path, err := projectcfg.Init(projectConfig.ProjectDir())
		if err != nil {
			return err
		}
		log.Infof("wrote %s", path)
		return nil
	},
}

var configDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Print what the ccbox- aliases expand to",
	RunE: func(_ *cobra.Command, _ []string) error {
		printutil.PrintSections(aliasSections())
		log.Info("")
		log.Infof("# %s: the providers above, combined", provider.AllProvidersAlias)
		log.Infof("# %s: the runtimes above, combined", runtime.AllRuntimesAlias)
		return nil
	},
}

// aliasField is one config field DefaultsAlias can appear in, with its expansion
type aliasField struct {
	fieldName string
	defaults  []string
}

// struct array keeps the order
var aliasFields = []aliasField{
	{"tmpfs_masks", projectcfg.TmpfsDefaults()},
	{"volume_masks", projectcfg.VolumeDefaults()},
	{"read_only_globs", projectcfg.ReadOnlyDefaults()},
	{"allowlist", projectcfg.AllowlistDefaults()},
}

// aliasSections are `ccbox config defaults`' sections: every ccbox- alias's
// expansion, in print order
func aliasSections() []printutil.Section {
	return slices.Concat(
		[]printutil.Section{{Header: projectcfg.DefaultsAlias + ":"}},
		uslice.Map(aliasFields, func(f aliasField) printutil.Section {
			return printutil.Section{Header: f.fieldName + ":", Items: f.defaults, Depth: 1}
		}),
		[]printutil.Section{{
			Header: projectcfg.SetHarnessAlias + " (cli " + *projectConfig.CLIName +
				"; a --vnc load adds the desktop's GUI app):",
			Items: projectConfig.SetHarnessDomains(),
		}},
		uslice.Map(provider.All(), func(p provider.Provider) printutil.Section {
			return printutil.Section{Header: p.Alias() + ":", Items: p.Domains}
		}),
		uslice.Map(runtime.All(), func(r runtime.Runtime) printutil.Section {
			return printutil.Section{Header: r.Alias() + ":", Items: r.Domains}
		}),
	)
}

var configUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Print the user-level config's path",
	RunE: func(_ *cobra.Command, _ []string) error {
		log.Info(userdir.Tilde(projectcfg.UserConfigFile()))
		return nil
	},
}

func init() { configCmd.AddCommand(configInitCmd, configDefaultsCmd, configUserCmd) }
