package exec

import (
	"context"
	"io"
)

// FakeRunner records calls and returns scripted output for tests.
type FakeRunner struct {
	Calls   []Command
	Handler func(c Command) (stdout string, err error)
}

// Run records c, writes any scripted stdout to c.Out, and returns the
// scripted error.
func (f *FakeRunner) Run(_ context.Context, c Command) error {
	f.Calls = append(f.Calls, c)
	if f.Handler == nil {
		return nil
	}
	out, err := f.Handler(c)
	if c.Out != nil && out != "" {
		io.WriteString(c.Out, out)
	}
	return err
}
