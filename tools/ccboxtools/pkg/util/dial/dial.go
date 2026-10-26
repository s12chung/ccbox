// Package dial waits out a listener's readiness: a start's gate proving its
// addr accepts connections before whatever dials it next.
package dial

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// The gate's cadence: an attempt dials no longer than dialTimeout, and failed
// attempts space out by poll.
const (
	dialTimeout = time.Second
	poll        = 100 * time.Millisecond
)

// Await waits until addr accepts a network connection, bounded by timeout —
// a start's readiness gate, so its consumers never race it.
func Await(ctx context.Context, network, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	dialer := &net.Dialer{Timeout: dialTimeout}
	var lastErr error
	for {
		conn, err := dialer.DialContext(ctx, network, addr)
		if err == nil {
			log.WarnErr("close probe dial", conn.Close())
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return fmt.Errorf("%s never accepted: %w", addr, ctx.Err())
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s never accepted: %w", addr, lastErr)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s never accepted: %w", addr, ctx.Err())
		case <-time.After(poll):
		}
	}
}
