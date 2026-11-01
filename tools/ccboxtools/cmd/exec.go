package cmd

import (
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/entrypoint"
)

var execCmd = cmdWithExec(cmdWithExecParams{
	Use:   "exec [--] [command...]",
	Short: "docker exec's no-proxy entry: the session arrives as the image's user — the egress wall's sessions land on `proxy exec` instead",
	Run:   entrypoint.Exec,
})
