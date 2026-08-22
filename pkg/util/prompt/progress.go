package prompt

import (
	"io"
	"os"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
)

// DisplayProgress renders a Docker JSON progress stream (e.g. an image pull) to
// stdout, drawing live progress bars when stdout is a terminal and falling back
// to plain lines otherwise.
func DisplayProgress(r io.Reader) error {
	outFd, isTerm := term.GetFdInfo(os.Stdout)
	return jsonmessage.DisplayJSONMessagesStream(r, os.Stdout, outFd, isTerm, nil)
}
