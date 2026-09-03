// Package config discovers stacks on disk (docker compose or native) and
// parses per-stack zook.yaml configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrStackNotFound is returned when a named stack has no matching directory.
var ErrStackNotFound = errors.New("stack not found")

// Stack is a single deployable stack (docker compose or native).
type Stack struct {
	Name        string
	Dir         string
	ComposeFile string       // "" for native-only stacks
	Runtime     string       // "docker" | "native"
	Config      *StackConfig // parsed zook.yaml; nil when absent
}

// composeNames are tried in order within a stack directory.
var composeNames = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

func composeFileIn(dir string) (string, bool) {
	for _, name := range composeNames {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, true
		}
	}
	return "", false
}

// stackAt builds a Stack for dir if it is a stack (has a compose file or
// zook.yaml). Returns ok=false when dir is not a stack.
func stackAt(name, dir string) (Stack, bool, error) {
	cf, hasCompose := composeFileIn(dir)
	cfg, err := LoadConfig(dir)
	if err != nil {
		return Stack{}, false, fmt.Errorf("%s: %w", name, err)
	}
	if !hasCompose && cfg == nil {
		return Stack{}, false, nil
	}
	return Stack{
		Name:        name,
		Dir:         dir,
		ComposeFile: cf, // "" when no compose file
		Runtime:     cfg.ResolvedRuntime(),
		Config:      cfg,
	}, true, nil
}

// isDir reports whether path is a directory, following symlinks. Stack
// directories are often symlinks into a repo checkout, and os.ReadDir reports
// those as non-directories — so listing must stat, not trust the dir entry.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// DiscoverStacks returns every stack under root, sorted by name.
func DiscoverStacks(root string) ([]Stack, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading stacks root %s: %w", root, err)
	}
	var stacks []Stack
	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if !isDir(path) {
			continue
		}
		s, ok, err := stackAt(e.Name(), path)
		if err != nil {
			return nil, err
		}
		if ok {
			stacks = append(stacks, s)
		}
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	return stacks, nil
}

// FindStack returns the named stack under root.
// It rejects names that are empty, equal to "." or "..", or that contain a
// path separator or ".." segment, preventing path-traversal attacks.
func FindStack(root, name string) (Stack, error) {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsRune(name, filepath.Separator) ||
		strings.Contains(name, "..") {
		return Stack{}, fmt.Errorf("%q: %w", name, ErrStackNotFound)
	}
	s, ok, err := stackAt(name, filepath.Join(root, name))
	if err != nil {
		return Stack{}, err
	}
	if !ok {
		return Stack{}, fmt.Errorf("%q: %w", name, ErrStackNotFound)
	}
	return s, nil
}
