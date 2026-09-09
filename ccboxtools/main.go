// Command ccboxtools maintains the container: it is the container's entrypoint —
// verifying identity and egress wall before exec'ing the command — and installs and
// maintains the devbox's coding CLI in the clis volume.
package main

import (
	"os"

	"github.com/s12chung/ccbox/ccboxtools/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
