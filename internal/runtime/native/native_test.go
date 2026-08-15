package native

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/exec"
	"github.com/oti-adjei/zook/internal/health"
)

// Health behavior is injected via a real *health.Prober backed by exec.OSRunner
// and a command health check ("true" = healthy, "false" = unhealthy) — no fakes
// needed for the prober itself.

func nativeStack(t *testing.T, cfg *config.StackConfig) config.Stack {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "releases", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	// stage the binary for v1
	if err := os.WriteFile(filepath.Join(dir, "releases", "v1", cfg.Binary), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return config.Stack{Name: "rue", Dir: dir, Runtime: "native", Config: cfg}
}

func healthyProber() *health.Prober { return health.New(nil, &exec.FakeRunner{}) }

// recordRunner records the commands it is asked to run (e.g. systemctl restart).
type recordRunner struct {
	calls []string
}

func (r *recordRunner) Run(_ context.Context, c exec.Command) error {
	r.calls = append(r.calls, c.Name+" "+joinArgs(c.Args))
	return nil
}
func joinArgs(a []string) string {
	out := ""
	for i, s := range a {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func cfgCommandHealth(binary string) *config.StackConfig {
	return &config.StackConfig{
		Runtime: "native", Binary: binary, SystemdUnit: "rue-api",
		Health: &config.HealthConfig{Command: []string{"true"}},
	}
}

func TestPreflightInvalidConfigErrors(t *testing.T) {
	rt := New(&recordRunner{}, healthyProber(), nil)
	s := config.Stack{Name: "rue", Dir: t.TempDir(), Runtime: "native",
		Config: &config.StackConfig{Runtime: "native"}} // missing binary/unit/health
	missing, err := rt.Preflight(context.Background(), s, io.Discard)
	if err == nil {
		t.Fatal("invalid native config must error from Preflight")
	}
	if len(missing) != 0 {
		t.Fatal("native must not use the missing-services channel")
	}
}

func TestPullVerifiesStagedRelease(t *testing.T) {
	cfg := cfgCommandHealth("rue-api")
	s := nativeStack(t, cfg)
	rt := New(&recordRunner{}, healthyProber(), nil)
	if err := rt.Pull(context.Background(), s, "v1", io.Discard); err != nil {
		t.Fatalf("staged release should pull-ok: %v", err)
	}
	// v2 is not staged and no artifact URL → error
	if err := rt.Pull(context.Background(), s, "v2", io.Discard); err == nil {
		t.Fatal("unstaged release with no artifact must error")
	}
}

func TestUpFlipsSymlinkRestartsAndProbes(t *testing.T) {
	cfg := cfgCommandHealth("rue-api")
	s := nativeStack(t, cfg)
	rr := &recordRunner{}
	// prober whose command health always passes: runner "true" exits 0
	prober := health.New(nil, exec.OSRunner{})
	rt := New(rr, prober, nil)

	err := rt.Up(context.Background(), s, "v1", 2*time.Second, io.Discard)
	if err != nil {
		t.Fatalf("Up should succeed: %v", err)
	}
	// current symlink points at releases/v1
	got, _ := os.Readlink(filepath.Join(s.Dir, "current"))
	if got != filepath.Join("releases", "v1") {
		t.Fatalf("current → %q, want releases/v1", got)
	}
	// systemctl restart was called for the unit
	found := false
	for _, c := range rr.calls {
		if c == "systemctl restart rue-api" {
			found = true
		}
	}
	if !found {
		t.Fatalf("systemctl restart not called; calls=%v", rr.calls)
	}
}

func TestUpFailsWhenUnhealthy(t *testing.T) {
	// health command "false" always fails → Up returns error after timeout
	cfg := &config.StackConfig{
		Runtime: "native", Binary: "rue-api", SystemdUnit: "rue-api",
		Health: &config.HealthConfig{Command: []string{"false"}},
	}
	s := nativeStack(t, cfg)
	rt := New(&recordRunner{}, health.New(nil, exec.OSRunner{}), nil)
	err := rt.Up(context.Background(), s, "v1", 30*time.Millisecond, io.Discard)
	if err == nil {
		t.Fatal("unhealthy app must make Up fail")
	}
}

func TestPullFetchesWhenArtifactSet(t *testing.T) {
	cfg := cfgCommandHealth("rue-api")
	cfg.Artifact = "https://x/rue-${VERSION}.tgz"
	dir := t.TempDir()
	s := config.Stack{Name: "rue", Dir: dir, Runtime: "native", Config: cfg}
	fx := &fakeFetcher{}
	rt := New(&recordRunner{}, healthyProber(), fx)
	if err := rt.Pull(context.Background(), s, "v9", io.Discard); err != nil {
		t.Fatalf("fetch pull: %v", err)
	}
	if fx.calls != 1 {
		t.Fatalf("fetcher should be called once, got %d", fx.calls)
	}
}

type fakeFetcher struct{ calls int }

func (f *fakeFetcher) Fetch(_ context.Context, urlTemplate, version, destDir string, _ io.Writer) error {
	f.calls++
	// simulate extraction: create the dir + binary
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destDir, "rue-api"), []byte("bin"), 0o755)
}
