package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverStacks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "alpha", "compose.yaml"))
	writeFile(t, filepath.Join(root, "beta", "compose.yml"))
	// a directory with no compose file must be ignored
	if err := os.MkdirAll(filepath.Join(root, "notastack"), 0o755); err != nil {
		t.Fatal(err)
	}

	stacks, err := DiscoverStacks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) != 2 {
		t.Fatalf("got %d stacks, want 2: %+v", len(stacks), stacks)
	}
	if stacks[0].Name != "alpha" || stacks[1].Name != "beta" {
		t.Fatalf("wrong names/order: %+v", stacks)
	}
	if filepath.Base(stacks[0].ComposeFile) != "compose.yaml" {
		t.Fatalf("wrong compose file: %s", stacks[0].ComposeFile)
	}
}

func TestFindStackMissing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "alpha", "compose.yaml"))

	_, err := FindStack(root, "nope")
	if !errors.Is(err, ErrStackNotFound) {
		t.Fatalf("err = %v, want ErrStackNotFound", err)
	}
}

func TestFindStackRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	// Write a compose file in the parent so a naive join would "find" it.
	writeFile(t, filepath.Join(root, "..", "compose.yaml"))

	cases := []string{
		"../../etc",
		"../sibling",
		"..",
		".",
		"",
		"a/b",
		"a" + string(filepath.Separator) + "b",
	}
	for _, name := range cases {
		_, err := FindStack(root, name)
		if !errors.Is(err, ErrStackNotFound) {
			t.Errorf("FindStack(root, %q) = %v, want ErrStackNotFound", name, err)
		}
	}
}
