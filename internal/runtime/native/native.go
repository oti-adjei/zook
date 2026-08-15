// Package native implements runtime.Runtime for prebuilt binaries managed by
// systemd. Deploy = stage/verify a release dir, flip the `current` symlink,
// restart the unit, and poll health. zook never builds or writes unit files.
package native

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/exec"
	"github.com/oti-adjei/zook/internal/health"
	"github.com/oti-adjei/zook/internal/runtime"
)

const pollInterval = 500 * time.Millisecond

// Fetcher downloads and extracts a version's artifact into destDir.
type Fetcher interface {
	Fetch(ctx context.Context, urlTemplate, version, destDir string, out io.Writer) error
}

// Runtime deploys native binaries under systemd.
type Runtime struct {
	runner  exec.Runner
	prober  *health.Prober
	fetcher Fetcher
}

// New returns a native Runtime.
func New(runner exec.Runner, prober *health.Prober, fetcher Fetcher) *Runtime {
	return &Runtime{runner: runner, prober: prober, fetcher: fetcher}
}

var _ runtime.Runtime = (*Runtime)(nil)

func (r *Runtime) releaseDir(s config.Stack, version string) string {
	return filepath.Join(s.Dir, "releases", version)
}

func healthConfig(c *config.StackConfig) health.Config {
	return health.Config{
		URL:            c.Health.URL,
		Command:        c.Health.Command,
		ExpectedStatus: c.Health.ExpectedStatus,
	}
}

// Preflight validates the native config. Problems are returned as an error so
// the engine refuses before touching state; the missing-services channel is
// unused for native.
func (r *Runtime) Preflight(_ context.Context, s config.Stack, _ io.Writer) ([]string, error) {
	if s.Config == nil {
		return nil, fmt.Errorf("stack %q has no zook.yaml for native runtime", s.Name)
	}
	if err := s.Config.Validate(); err != nil {
		return nil, err
	}
	return nil, nil
}

// Pull ensures the release for version exists on disk (fetching it when an
// artifact URL is configured), and that the binary is present.
func (r *Runtime) Pull(ctx context.Context, s config.Stack, version string, out io.Writer) error {
	dir := r.releaseDir(s, version)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if s.Config.Artifact == "" {
			return fmt.Errorf("release %q not staged at %s and no artifact URL configured", version, dir)
		}
		if r.fetcher == nil {
			return fmt.Errorf("artifact configured but no fetcher available")
		}
		if err := r.fetcher.Fetch(ctx, s.Config.Artifact, version, dir, out); err != nil {
			return fmt.Errorf("fetching artifact: %w", err)
		}
	}
	bin := filepath.Join(dir, s.Config.Binary)
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("binary %q missing in release %q: %w", s.Config.Binary, version, err)
	}
	return nil
}

// Up flips current → releases/version, restarts the unit, and waits for health.
func (r *Runtime) Up(ctx context.Context, s config.Stack, version string, timeout time.Duration, out io.Writer) error {
	if err := flipCurrent(s.Dir, version); err != nil {
		return fmt.Errorf("activating release %q: %w", version, err)
	}
	fmt.Fprintf(out, "current -> releases/%s\n", version)
	if err := r.runner.Run(ctx, exec.Command{Name: "systemctl", Args: []string{"restart", s.Config.SystemdUnit}, Out: out}); err != nil {
		return fmt.Errorf("systemctl restart %s: %w", s.Config.SystemdUnit, err)
	}
	if err := r.prober.Poll(ctx, healthConfig(s.Config), timeout, pollInterval); err != nil {
		return err
	}
	return nil
}

// Health reports whether the app currently passes its health check.
func (r *Runtime) Health(ctx context.Context, s config.Stack) (bool, error) {
	if s.Config == nil {
		return false, nil
	}
	return r.prober.Check(ctx, healthConfig(s.Config)) == nil, nil
}

// flipCurrent atomically points <stackDir>/current at releases/<version>.
func flipCurrent(stackDir, version string) error {
	target := filepath.Join("releases", version) // relative link
	link := filepath.Join(stackDir, "current")
	tmp := link + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, link)
}

// expandVersion substitutes ${VERSION} in a URL/template.
func expandVersion(tmpl, version string) string {
	return strings.ReplaceAll(tmpl, "${VERSION}", version)
}
