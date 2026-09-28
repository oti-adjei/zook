// Package wire builds the engines and runtimes shared by zook's CLI and HTTP
// transports: it resolves environment defaults, selects the runtime a stack's
// zook.yaml asks for, and constructs the release engine for it.
package wire

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
	"github.com/oti-adjei/zook/internal/exec"
	"github.com/oti-adjei/zook/internal/health"
	"github.com/oti-adjei/zook/internal/runtime"
	"github.com/oti-adjei/zook/internal/runtime/docker"
	"github.com/oti-adjei/zook/internal/runtime/native"
)

// StacksRoot returns the directory stacks live under (ZOOK_STACKS_ROOT,
// default /opt/stacks).
func StacksRoot() string {
	if r := os.Getenv("ZOOK_STACKS_ROOT"); r != "" {
		return r
	}
	return "/opt/stacks"
}

// DefaultTimeout returns the health-wait timeout (ZOOK_TIMEOUT in seconds,
// default 60s). Per-stack health_timeout overrides still apply.
func DefaultTimeout() time.Duration {
	if s := os.Getenv("ZOOK_TIMEOUT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

// RuntimeFor returns the deployment runtime a stack's config selects.
func RuntimeFor(s config.Stack) runtime.Runtime {
	if s.Runtime == "native" {
		prober := health.New(&http.Client{}, exec.OSRunner{})
		return native.New(exec.OSRunner{}, prober, native.NewHTTPFetcher(&http.Client{}))
	}
	return docker.New(exec.OSRunner{})
}

// EngineFor builds the release engine for a stack, with the per-stack
// health-wait timeout (falling back to defTimeout) and rollback override.
func EngineFor(s config.Stack, defTimeout time.Duration) *core.Engine {
	to := s.Config.Timeout(defTimeout)
	return core.NewEngine(RuntimeFor(s), to, core.WithRollbackOnFail(s.Config.ShouldRollback()))
}
