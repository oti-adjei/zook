package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderAllGolden(t *testing.T) {
	root := t.TempDir()
	hb := filepath.Join(root, "handbook")
	content := filepath.Join(root, "content")
	write(t, filepath.Join(hb, "README.md"), "# Overview\n\nSee [concepts](concepts.md).\n")
	write(t, filepath.Join(hb, "concepts.md"), "# Concepts\n\n| A | B |\n|---|---|\n| 1 | 2 |\n")
	write(t, filepath.Join(content, "install.md"), "# Install\n\nGrab the binary.\n")
	write(t, filepath.Join(root, "CHANGELOG.md"), "# Changelog\n\n- first\n")

	g := &Generator{
		HandbookDir:   hb,
		ContentDir:    content,
		TemplateDir:   "../templates",
		StaticDir:     "../static",
		ChangelogPath: filepath.Join(root, "CHANGELOG.md"),
		OutDir:        filepath.Join(root, "dist"),
		Nav: []NavItem{
			{Slug: "overview", Title: "Overview", File: "README.md"},
			{Slug: "concepts", Title: "Concepts", File: "concepts.md"},
		},
	}
	warnings, err := g.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	must := func(rel, needle string) {
		b, err := os.ReadFile(filepath.Join(g.OutDir, rel))
		if err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
		if needle != "" && !strings.Contains(string(b), needle) {
			t.Fatalf("%s missing %q", rel, needle)
		}
	}
	must("index.html", "<title>")                              // landing
	must("docs/overview/index.html", `href="/docs/concepts/"`) // rewritten cross-link
	must("docs/concepts/index.html", "<table>")                // GFM table
	must("install/index.html", "Grab the binary")
	must("changelog/index.html", "first")
	must("static/style.css", "") // static copied
}
