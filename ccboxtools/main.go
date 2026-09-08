// Command ccboxtools installs and maintains the devbox's coding CLI in the clis volume.
package main

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
