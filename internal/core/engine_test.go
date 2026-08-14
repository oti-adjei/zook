package core

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/oti-adjei/zook/internal/config"
)

type stubRuntime struct {
	missing   []string
	upErrs    []error // consumed per Up call
	upCalls   []string
	pullCalls int
}

func (s *stubRuntime) Preflight(_ context.Context, _ config.Stack, _ io.Writer) ([]string, error) {
	return s.missing, nil
}
func (s *stubRuntime) Pull(_ context.Context, _ config.Stack, _ string, _ io.Writer) error {
	s.pullCalls++
	return nil
}
func (s *stubRuntime) Up(_ context.Context, _ config.Stack, version string, _ time.Duration, _ io.Writer) error {
	s.upCalls = append(s.upCalls, version)
	var err error
	if len(s.upErrs) > 0 {
		err, s.upErrs = s.upErrs[0], s.upErrs[1:]
	}
	return err
}
func (s *stubRuntime) Health(_ context.Context, _ config.Stack) (bool, error) { return true, nil }

func newStack(t *testing.T) config.Stack {
	dir := t.TempDir()
	return config.Stack{Name: "app", Dir: dir, ComposeFile: dir + "/compose.yaml"}
}

func fixedNow(e *Engine) { e.now = func() time.Time { return time.Unix(0, 0).UTC() } }

func TestDeployRefusesOnMissingHealthcheck(t *testing.T) {
	rt := &stubRuntime{missing: []string{"tenant-api"}}
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	err := e.Deploy(context.Background(), newStack(t), "v1")
	if err == nil || !strings.Contains(err.Error(), "tenant-api") {
		t.Fatalf("want refusal naming tenant-api, got %v", err)
	}
	if len(rt.upCalls) != 0 {
		t.Fatalf("Up must not be called on refusal")
	}
}

func TestDeploySuccessRotatesState(t *testing.T) {
	rt := &stubRuntime{}
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	s := newStack(t)
	_ = SaveState(s.Dir, State{Current: "v1"})
	if err := e.Deploy(context.Background(), s, "v2"); err != nil {
		t.Fatal(err)
	}
	st, _ := LoadState(s.Dir)
	if st.Current != "v2" || st.Previous != "v1" {
		t.Fatalf("state = %+v", st)
	}
}

func TestDeployAutoRollsBackOnUpFailure(t *testing.T) {
	rt := &stubRuntime{upErrs: []error{errors.New("unhealthy"), nil}} // new fails, rollback ok
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	s := newStack(t)
	_ = SaveState(s.Dir, State{Current: "v1"})
	err := e.Deploy(context.Background(), s, "v2")
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("want rolled-back error, got %v", err)
	}
	if len(rt.upCalls) != 2 || rt.upCalls[1] != "v1" {
		t.Fatalf("expected rollback to v1, calls=%v", rt.upCalls)
	}
	st, _ := LoadState(s.Dir)
	if st.Current != "v1" {
		t.Fatalf("current must stay v1, got %s", st.Current)
	}
	if st.History[len(st.History)-1].Result != ResultRolledBack {
		t.Fatalf("last history should be rolled_back: %+v", st.History)
	}
}

func TestDeployRollbackAlsoFailsNeedsManual(t *testing.T) {
	rt := &stubRuntime{upErrs: []error{errors.New("unhealthy"), errors.New("still broken")}}
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	s := newStack(t)
	_ = SaveState(s.Dir, State{Current: "v1"})
	err := e.Deploy(context.Background(), s, "v2")
	if err == nil || !strings.Contains(err.Error(), "manual intervention") {
		t.Fatalf("want manual-intervention error, got %v", err)
	}
	st, _ := LoadState(s.Dir)
	if st.History[len(st.History)-1].Result != ResultFailed {
		t.Fatalf("last history should be failed: %+v", st.History)
	}
}

func TestRollbackWithoutPreviousErrors(t *testing.T) {
	rt := &stubRuntime{}
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	s := newStack(t)
	_ = SaveState(s.Dir, State{Current: "v1"})
	if err := e.Rollback(context.Background(), s); err == nil {
		t.Fatal("expected error when no previous version")
	}
	if len(rt.upCalls) != 0 {
		t.Fatal("Up must not be called")
	}
}

func TestRollbackSwapsCurrentPrevious(t *testing.T) {
	rt := &stubRuntime{}
	e := NewEngine(rt, time.Second)
	fixedNow(e)
	s := newStack(t)
	_ = SaveState(s.Dir, State{Current: "v2", Previous: "v1"})
	if err := e.Rollback(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	st, _ := LoadState(s.Dir)
	if st.Current != "v1" || st.Previous != "v2" {
		t.Fatalf("state = %+v", st)
	}
}
