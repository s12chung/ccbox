package cmd

import (
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/entrypoint"
)

var execCmd = cmdWithExec(cmdWithExecParams{
	Use:   "exec [--] [command...]",
	Short: "docker exec's entry: an egress wall session arrives root and crosses the netns; no-proxy arrives --user ccbox",
	Run:   entrypoint.Exec,
})
