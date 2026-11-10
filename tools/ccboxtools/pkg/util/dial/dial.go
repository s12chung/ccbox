// Package dial waits out a listener's readiness: a start's gate proving its
// addr listens before whatever dials it next.
package dial

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	poll      = 100 * time.Millisecond
	tcpListen = "0A"
)

// AwaitListen waits until procfs shows a socket LISTENing on port in the pid's
// netns (0 = this process), bounded by timeout — no dial, so the probe never logs.
func AwaitListen(ctx context.Context, pid int, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if listening(pid, port) {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("port %d never listened: %w", port, ctx.Err())
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port %d never listened", port)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("port %d never listened: %w", port, ctx.Err())
		case <-time.After(poll):
		}
	}
}

// listening reports whether procfs shows a socket LISTENing on port in the
// pid's netns — its /proc/<pid>/net tcp tables; 0 names this process's own.
func listening(pid, port int) bool {
	if pid == 0 {
		pid = os.Getpid()
	}
	for _, name := range []string{"tcp", "tcp6"} {
		body, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/net/" + name) //nolint:gosec // the fixed procfs path
		if err != nil {
			continue // a missing table holds no listener
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 4 || fields[3] != tcpListen {
				continue
			}
			_, portHex, found := strings.Cut(fields[1], ":")
			if !found {
				continue // the table's header row
			}
			if parsed, err := strconv.ParseUint(portHex, 16, 16); err == nil && int(parsed) == port {
				return true
			}
		}
	}
	return false
}
