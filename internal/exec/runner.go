// Package exec runs external commands behind an injectable interface so
// callers can be unit-tested with a fake.
package exec

import (
	"context"
	"io"
	"os"
	osexec "os/exec"
)

// Command describes a single external command invocation.
type Command struct {
	Dir  string   // working directory (empty = inherit)
	Env  []string // extra "KEY=VALUE" entries, appended to os.Environ
	Name string
	Args []string
	Out  io.Writer // combined stdout+stderr sink (may be nil)
}

// Runner executes a Command.
type Runner interface {
	Run(ctx context.Context, c Command) error
}

// OSRunner runs commands as real OS processes.
type OSRunner struct{}

// Run executes c, returning a non-nil error on non-zero exit.
func (OSRunner) Run(ctx context.Context, c Command) error {
	cmd := osexec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	if c.Out != nil {
		cmd.Stdout = c.Out
		cmd.Stderr = c.Out
	}
	return cmd.Run()
}
