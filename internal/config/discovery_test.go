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

func TestDiscoverIncludesNativeStacks(t *testing.T) {
	root := t.TempDir()
	// docker stack
	writeFile(t, filepath.Join(root, "web", "compose.yaml"))
	// native stack: zook.yaml, no compose
	writeZook(t, filepath.Join(root, "rue"),
		"runtime: native\nbinary: rue-api\nsystemd_unit: rue-api\nhealth:\n  url: http://localhost:8080/healthz\n")

	stacks, err := DiscoverStacks(root)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Stack{}
	for _, s := range stacks {
		byName[s.Name] = s
	}
	if len(stacks) != 2 {
		t.Fatalf("want 2 stacks, got %d: %v", len(stacks), stacks)
	}
	if byName["web"].Runtime != "docker" {
		t.Fatalf("web should be docker, got %q", byName["web"].Runtime)
	}
	rue := byName["rue"]
	if rue.Runtime != "native" || rue.Config == nil || rue.ComposeFile != "" {
		t.Fatalf("rue native resolution wrong: %+v", rue)
	}
}

func TestFindStackNativeOnly(t *testing.T) {
	root := t.TempDir()
	writeZook(t, filepath.Join(root, "rue"),
		"runtime: native\nbinary: rue-api\nsystemd_unit: rue-api\nhealth:\n  command: [rue-api, health]\n")
	s, err := FindStack(root, "rue")
	if err != nil {
		t.Fatal(err)
	}
	if s.Runtime != "native" || s.Config == nil {
		t.Fatalf("native stack not resolved: %+v", s)
	}
}

func TestFindStackStillGuardsTraversal(t *testing.T) {
	root := t.TempDir()
	writeZook(t, filepath.Join(root, "rue"), "runtime: native\nbinary: b\nsystemd_unit: u\nhealth:\n  url: http://x/h\n")
	if _, err := FindStack(root, "../rue"); !errors.Is(err, ErrStackNotFound) {
		t.Fatalf("traversal must be rejected: %v", err)
	}
}
