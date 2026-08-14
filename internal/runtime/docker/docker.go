// Package docker implements runtime.Runtime by shelling out to `docker compose`.
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/exec"
)

// Runtime shells out to the docker CLI through an exec.Runner.
type Runtime struct{ r exec.Runner }

// New returns a docker Runtime backed by r.
func New(r exec.Runner) *Runtime { return &Runtime{r: r} }

func (d *Runtime) cmd(s config.Stack, version string, out io.Writer, args ...string) exec.Command {
	full := append([]string{"compose", "-f", s.ComposeFile}, args...)
	c := exec.Command{Dir: s.Dir, Name: "docker", Args: full, Out: out}
	if version != "" {
		c.Env = []string{"VERSION=" + version}
	}
	return c
}

// Preflight lists active services whose config has no usable healthcheck.
func (d *Runtime) Preflight(ctx context.Context, s config.Stack, out io.Writer) ([]string, error) {
	var svcBuf bytes.Buffer
	if err := d.r.Run(ctx, d.cmd(s, "", &svcBuf, "config", "--services")); err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}
	var cfgBuf bytes.Buffer
	if err := d.r.Run(ctx, d.cmd(s, "", &cfgBuf, "config", "--format", "json")); err != nil {
		return nil, fmt.Errorf("reading compose config: %w", err)
	}
	var cfg struct {
		Services map[string]struct {
			Healthcheck *struct {
				Disable bool `json:"disable"`
			} `json:"healthcheck"`
		} `json:"services"`
	}
	if err := json.Unmarshal(cfgBuf.Bytes(), &cfg); err != nil {
		return nil, fmt.Errorf("parsing compose config: %w", err)
	}
	var missing []string
	for _, name := range strings.Fields(svcBuf.String()) {
		svc, ok := cfg.Services[name]
		if !ok || svc.Healthcheck == nil || svc.Healthcheck.Disable {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// Pull fetches images for version.
func (d *Runtime) Pull(ctx context.Context, s config.Stack, version string, out io.Writer) error {
	return d.r.Run(ctx, d.cmd(s, version, out, "pull"))
}

// Up recreates the stack at version and waits for health.
func (d *Runtime) Up(ctx context.Context, s config.Stack, version string, timeout time.Duration, out io.Writer) error {
	secs := strconv.Itoa(int(timeout.Seconds()))
	return d.r.Run(ctx, d.cmd(s, version, out, "up", "-d", "--wait", "--wait-timeout", secs))
}

// Health reports whether every active service is running and healthy.
func (d *Runtime) Health(ctx context.Context, s config.Stack) (bool, error) {
	var buf bytes.Buffer
	if err := d.r.Run(ctx, d.cmd(s, "", &buf, "ps", "--format", "json")); err != nil {
		return false, err
	}
	dec := json.NewDecoder(strings.NewReader(buf.String()))
	seen := false
	for dec.More() {
		var row struct{ State, Health string }
		if err := dec.Decode(&row); err != nil {
			return false, err
		}
		seen = true
		if row.State != "running" {
			return false, nil
		}
		if row.Health != "" && row.Health != "healthy" {
			return false, nil
		}
	}
	return seen, nil
}
