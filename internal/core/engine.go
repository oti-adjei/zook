package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/runtime"
)

// Engine orchestrates deploys and rollbacks over a runtime.Runtime.
type Engine struct {
	rt             runtime.Runtime
	timeout        time.Duration
	now            func() time.Time
	rollbackOnFail bool
}

// Option customizes an Engine.
type Option func(*Engine)

// WithRollbackOnFail toggles automatic rollback on a failed deploy (default true).
func WithRollbackOnFail(v bool) Option {
	return func(e *Engine) { e.rollbackOnFail = v }
}

// NewEngine returns an Engine using rt and the given health-wait timeout.
func NewEngine(rt runtime.Runtime, timeout time.Duration, opts ...Option) *Engine {
	e := &Engine{rt: rt, timeout: timeout, now: time.Now, rollbackOnFail: true}
	for _, o := range opts {
		o(e)
	}
	return e
}

func (e *Engine) openLog(stackDir, version string) (io.WriteCloser, error) {
	dir := filepath.Join(stackDir, ".zook", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s.log", e.now().UTC().Format("20060102T150405Z"), version)
	return os.Create(filepath.Join(dir, name))
}

// Deploy runs preflight, pull, up-with-health-wait, and auto-rollback on failure.
func (e *Engine) Deploy(ctx context.Context, s config.Stack, version string) error {
	st, err := LoadState(s.Dir)
	if err != nil {
		return err
	}
	log, err := e.openLog(s.Dir, version)
	if err != nil {
		return err
	}
	defer log.Close()
	fmt.Fprintf(log, "=== deploy %s %s @ %s ===\n", s.Name, version, e.now().UTC().Format(time.RFC3339))

	missing, err := e.rt.Preflight(ctx, s, log)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if len(missing) > 0 {
		msg := fmt.Sprintf("refusing deploy: services missing a healthcheck: %s", strings.Join(missing, ", "))
		fmt.Fprintln(log, msg)
		return fmt.Errorf("%s", msg)
	}

	if err := e.rt.Pull(ctx, s, version, log); err != nil {
		return fmt.Errorf("pull: %w", err)
	}

	upErr := e.rt.Up(ctx, s, version, e.timeout, log)
	if upErr == nil {
		st.recordSuccess(version, e.now())
		return SaveState(s.Dir, st)
	}
	fmt.Fprintf(log, "deploy of %s failed: %v\n", version, upErr)

	if !e.rollbackOnFail {
		st.recordFailed(version, e.now())
		_ = SaveState(s.Dir, st)
		fmt.Fprintln(log, "auto-rollback disabled (rollback_on_fail: false)")
		return fmt.Errorf("deploy of %s failed (auto-rollback disabled): %w", version, upErr)
	}

	if st.Current == "" {
		st.recordFailed(version, e.now())
		_ = SaveState(s.Dir, st)
		return fmt.Errorf("deploy failed and no previous version to roll back to: %w", upErr)
	}

	fmt.Fprintf(log, "rolling back to %s\n", st.Current)
	if rbErr := e.rt.Up(ctx, s, st.Current, e.timeout, log); rbErr != nil {
		st.recordFailed(version, e.now())
		_ = SaveState(s.Dir, st)
		fmt.Fprintln(log, "ROLLBACK FAILED — manual intervention required")
		return fmt.Errorf("deploy failed and rollback failed — manual intervention required (deploy: %v; rollback: %v)", upErr, rbErr)
	}
	oldCurrent := st.Current
	st.recordRolledBack(version, e.now())
	_ = SaveState(s.Dir, st)
	return fmt.Errorf("deploy of %s failed, rolled back to %s: %w", version, oldCurrent, upErr)
}

// Rollback restores the previous version, health-gated.
func (e *Engine) Rollback(ctx context.Context, s config.Stack) error {
	st, err := LoadState(s.Dir)
	if err != nil {
		return err
	}
	if st.Previous == "" {
		return fmt.Errorf("stack %q has no previous version to roll back to", s.Name)
	}
	target := st.Previous
	log, err := e.openLog(s.Dir, target)
	if err != nil {
		return err
	}
	defer log.Close()
	fmt.Fprintf(log, "=== rollback %s -> %s ===\n", s.Name, target)

	if err := e.rt.Up(ctx, s, target, e.timeout, log); err != nil {
		st.recordFailed(target, e.now())
		_ = SaveState(s.Dir, st)
		return fmt.Errorf("rollback to %s failed: %w", target, err)
	}
	st.recordSuccess(target, e.now())
	return SaveState(s.Dir, st)
}
