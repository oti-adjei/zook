// Package runtime defines the seam between Zook's engine and a concrete
// deployment backend. V1 ships the docker subpackage.
package runtime

import (
	"context"
	"io"
	"time"

	"github.com/oti-adjei/zook/internal/config"
)

// Runtime drives a concrete deployment backend for one stack.
type Runtime interface {
	// Preflight returns active services that lack a usable healthcheck.
	Preflight(ctx context.Context, s config.Stack, out io.Writer) (missing []string, err error)
	// Pull fetches images for the given version.
	Pull(ctx context.Context, s config.Stack, version string, out io.Writer) error
	// Up starts/recreates the stack at version and blocks until healthy or timeout.
	Up(ctx context.Context, s config.Stack, version string, timeout time.Duration, out io.Writer) error
	// Health reports whether all active services are currently healthy.
	Health(ctx context.Context, s config.Stack) (healthy bool, err error)
}
