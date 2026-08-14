// Package config discovers Compose stacks on disk. A stack is any immediate
// subdirectory of the stacks root that contains a Compose file.
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

// Stack is a single deployable Compose project.
type Stack struct {
	Name        string
	Dir         string
	ComposeFile string
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

// DiscoverStacks returns every stack under root, sorted by name.
func DiscoverStacks(root string) ([]Stack, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading stacks root %s: %w", root, err)
	}
	var stacks []Stack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if cf, ok := composeFileIn(dir); ok {
			stacks = append(stacks, Stack{Name: e.Name(), Dir: dir, ComposeFile: cf})
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
	dir := filepath.Join(root, name)
	cf, ok := composeFileIn(dir)
	if !ok {
		return Stack{}, fmt.Errorf("%q: %w", name, ErrStackNotFound)
	}
	return Stack{Name: name, Dir: dir, ComposeFile: cf}, nil
}
