package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List this project's devboxes and their live sessions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		statuses, err := docker.DevboxStatuses(dock.MustNewCtxD(cmd.Context()), config.ProjectDir())
		if err != nil {
			return err
		}
		return printDevboxStatuses(statuses)
	},
}

// printDevboxStatuses prints the statuses as a docker ps-style table
func printDevboxStatuses(statuses []docker.DevboxStatus) error {
	if len(statuses) == 0 {
		log.Info("no devboxes for this project — boot one with `ccbox proxy` or `ccbox exec`")
		return nil
	}

	out := &bytes.Buffer{}
	rows := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(rows, "NAME\tSTATE\tEXECS\n"); err != nil {
		return err
	}
	for _, s := range statuses {
		name := s.Name
		if s.Foreground {
			name += " (foregrounded)"
		}
		if _, err := fmt.Fprintf(rows, "%s\t%s\t%d\n", name, s.State, s.Execs); err != nil {
			return err
		}
	}
	if err := rows.Flush(); err != nil {
		return err
	}
	log.Info(strings.TrimRight(out.String(), "\n"))
	return nil
}
