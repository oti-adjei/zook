// Package cli dispatches zook subcommands.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
	"github.com/oti-adjei/zook/internal/exec"
	"github.com/oti-adjei/zook/internal/runtime/docker"
)

const usage = `usage: zook <command> [args]

commands:
  deploy    <stack> <version>   deploy a version, auto-rollback on failure
  rollback  <stack>             restore the previous version
  status    [stack]             show current version and health
  releases  <stack>             show release history
  list                          list discovered stacks
  logs      <stack>             show recent deploy logs
`

func stacksRoot() string {
	if r := os.Getenv("ZOOK_STACKS_ROOT"); r != "" {
		return r
	}
	return "/opt/stacks"
}

func timeout() time.Duration {
	if s := os.Getenv("ZOOK_TIMEOUT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

func newEngine() *core.Engine {
	return core.NewEngine(docker.New(exec.OSRunner{}), timeout())
}

// Run dispatches a subcommand and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	ctx := context.Background()
	switch args[0] {
	case "deploy":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "usage: zook deploy <stack> <version>")
			return 2
		}
		s, err := config.FindStack(stacksRoot(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := newEngine().Deploy(ctx, s, args[2]); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "deployed %s %s\n", s.Name, args[2])
		return 0

	case "rollback":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: zook rollback <stack>")
			return 2
		}
		s, err := config.FindStack(stacksRoot(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := newEngine().Rollback(ctx, s); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "rolled back %s\n", s.Name)
		return 0

	case "status":
		return cmdStatus(args[1:], stdout, stderr)

	case "releases":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: zook releases <stack>")
			return 2
		}
		st, err := core.LoadState(stackDir(args[1]))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for i := len(st.History) - 1; i >= 0; i-- {
			h := st.History[i]
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", h.Version, h.Timestamp.Format(time.RFC3339), h.Result)
		}
		return 0

	case "list":
		stacks, err := config.DiscoverStacks(stacksRoot())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, s := range stacks {
			fmt.Fprintln(stdout, s.Name)
		}
		return 0

	case "logs":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: zook logs <stack>")
			return 2
		}
		return cmdLogs(args[1], stdout, stderr)

	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func stackDir(name string) string {
	return filepath.Join(stacksRoot(), name)
}
