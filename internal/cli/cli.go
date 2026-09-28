// Package cli dispatches zook subcommands.
package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
	"github.com/oti-adjei/zook/internal/wire"
)

const usage = `usage: zook <command> [args]

commands:
  deploy    <stack> <version>   deploy a version, auto-rollback on failure
  rollback  <stack>             restore the previous version
  status    [stack]             show current version and health
  releases  <stack>             show release history
  list                          list discovered stacks
  logs      <stack>             show recent deploy logs
  serve                       run the HTTP API + webhook daemon (env: ZOOK_ADDR,
                              ZOOK_API_TOKEN, ZOOK_WEBHOOK_SECRET)
  version                       print zook's version
`

func engineFor(s config.Stack) *core.Engine {
	return wire.EngineFor(s, wire.DefaultTimeout())
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
		s, err := config.FindStack(wire.StacksRoot(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := engineFor(s).Deploy(ctx, s, args[2]); err != nil {
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
		s, err := config.FindStack(wire.StacksRoot(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := engineFor(s).Rollback(ctx, s); err != nil {
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
		s, err := config.FindStack(wire.StacksRoot(), args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		st, err := core.LoadState(s.Dir)
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
		stacks, err := config.DiscoverStacks(wire.StacksRoot())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "STACK\tRUNTIME")
		for _, s := range stacks {
			fmt.Fprintf(w, "%s\t%s\n", s.Name, s.Runtime)
		}
		w.Flush()
		return 0

	case "logs":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: zook logs <stack>")
			return 2
		}
		return cmdLogs(args[1], stdout, stderr)

	case "serve":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: zook serve")
			return 2
		}
		return cmdServe(stdout, stderr)

	case "version":
		printVersion(stdout)
		return 0

	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
