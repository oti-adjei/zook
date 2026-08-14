package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
	"github.com/oti-adjei/zook/internal/exec"
	"github.com/oti-adjei/zook/internal/runtime/docker"
)

func cmdStatus(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: zook status [stack]")
		return 2
	}
	root := stacksRoot()
	var stacks []config.Stack
	if len(args) == 1 {
		s, err := config.FindStack(root, args[0])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		stacks = []config.Stack{s}
	} else {
		var err error
		if stacks, err = config.DiscoverStacks(root); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	rt := docker.New(exec.OSRunner{})
	w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "STACK\tVERSION\tSTATUS")
	for _, s := range stacks {
		st, _ := core.LoadState(s.Dir)
		status := "unknown"
		if ok, err := rt.Health(context.Background(), s); err == nil {
			if ok {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}
		version := st.Current
		if version == "" {
			version = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, version, status)
	}
	return flush(w)
}

func flush(w *tabwriter.Writer) int {
	if err := w.Flush(); err != nil {
		return 1
	}
	return 0
}

func cmdLogs(name string, stdout, stderr io.Writer) int {
	dir := filepath.Join(stacksRoot(), name, ".zook", "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(stderr, "no logs for %q\n", name)
		return 1
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Fprintf(stderr, "no logs for %q\n", name)
		return 1
	}
	newest := names[len(names)-1]
	data, err := os.ReadFile(filepath.Join(dir, newest))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "== %s ==\n", newest)
	stdout.Write(data)
	return 0
}
