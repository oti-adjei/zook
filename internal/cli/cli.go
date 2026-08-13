// Package cli dispatches zook subcommands.
package cli

import (
	"fmt"
	"io"
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

// Run dispatches a subcommand and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
