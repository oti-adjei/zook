// Package health probes whether an application is healthy, by HTTP GET or by
// running a command. It is the health engine the native runtime needs, since
// there is no Docker healthcheck to wait on.
package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/oti-adjei/zook/internal/exec"
)

// Config declares one health strategy. Exactly one of URL / Command is set.
type Config struct {
	URL            string
	Command        []string
	ExpectedStatus int // 0 → 200
}

// Doer performs an HTTP request. *http.Client satisfies it.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Prober runs health checks via an injectable HTTP client and command runner.
type Prober struct {
	doer   Doer
	runner exec.Runner
}

// New returns a Prober.
func New(doer Doer, runner exec.Runner) *Prober {
	return &Prober{doer: doer, runner: runner}
}

// Check runs a single probe. A nil error means healthy.
func (p *Prober) Check(ctx context.Context, cfg Config) error {
	if cfg.URL != "" {
		want := cfg.ExpectedStatus
		if want == 0 {
			want = 200
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
		if err != nil {
			return err
		}
		resp, err := p.doer.Do(req)
		if err != nil {
			return err
		}
		if resp.Body != nil {
			resp.Body.Close()
		}
		if resp.StatusCode != want {
			return fmt.Errorf("health GET %s: status %d, want %d", cfg.URL, resp.StatusCode, want)
		}
		return nil
	}
	return p.runner.Run(ctx, exec.Command{Name: cfg.Command[0], Args: cfg.Command[1:]})
}

// Poll repeats Check every interval until it succeeds or timeout elapses,
// returning the last failure on timeout.
func (p *Prober) Poll(ctx context.Context, cfg Config, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		last = p.Check(ctx, cfg)
		if last == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health never passed within %s: %w", timeout, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
