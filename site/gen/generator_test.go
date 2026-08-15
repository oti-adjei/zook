package main

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestGen(t *testing.T) *Generator {
	t.Helper()
	g := &Generator{
		TemplateDir: "../templates",
		OutDir:      t.TempDir(),
		Nav:         docsNav,
	}
	if err := g.loadTemplates(); err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
	return g
}

func TestRenderDocPage(t *testing.T) {
	g := newTestGen(t)
	err := g.renderPage("doc", "docs/concepts", pageData{
		Title: "Concepts", Section: "docs",
		Body: template.HTML("<p>hello body</p>"),
		Nav:  docsNav, ActiveSlug: "concepts",
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(g.OutDir, "docs", "concepts", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<title>Concepts") {
		t.Fatalf("title not in <title>: %q", s)
	}
	if !strings.Contains(s, "hello body") {
		t.Fatalf("body not rendered: %q", s)
	}
	if !strings.Contains(s, `/docs/concepts/`) || !strings.Contains(s, "Native Runtime") {
		t.Fatalf("sidebar nav missing: %q", s)
	}
}

func TestRenderLandingAtRoot(t *testing.T) {
	g := newTestGen(t)
	if err := g.renderPage("landing", "", pageData{Title: "zook", Section: "landing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(g.OutDir, "index.html")); err != nil {
		t.Fatalf("landing index.html not at root: %v", err)
	}
}
