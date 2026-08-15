package main

import (
	"strings"
	"testing"
)

func TestExtractTitle(t *testing.T) {
	title, body := extractTitle([]byte("# Concepts\n\nSome text\n"))
	if title != "Concepts" {
		t.Fatalf("title = %q, want Concepts", title)
	}
	if strings.Contains(string(body), "# Concepts") {
		t.Fatalf("body should not contain the H1 line: %q", body)
	}
	if !strings.Contains(string(body), "Some text") {
		t.Fatalf("body lost its content: %q", body)
	}
}

func TestExtractTitleNoH1(t *testing.T) {
	title, body := extractTitle([]byte("no heading here\n"))
	if title != "" || !strings.Contains(string(body), "no heading") {
		t.Fatalf("no-H1 case wrong: title=%q body=%q", title, body)
	}
}

func TestRenderMarkdownTable(t *testing.T) {
	// GFM tables must render (handbook uses them).
	out, err := renderMarkdown([]byte("| A | B |\n|---|---|\n| 1 | 2 |\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<table>") {
		t.Fatalf("GFM table not rendered: %q", out)
	}
}

func TestRewriteDocLinks(t *testing.T) {
	routes := routeMap()
	in := `<p>see <a href="concepts.md">concepts</a> and <a href="native.md#health">health</a> and <a href="../CHANGELOG.md">log</a></p>`
	out, warnings := rewriteDocLinks(in, routes)
	if !strings.Contains(out, `href="/docs/concepts/"`) {
		t.Fatalf("concepts link not rewritten: %q", out)
	}
	if !strings.Contains(out, `href="/docs/native/#health"`) {
		t.Fatalf("anchor link not rewritten: %q", out)
	}
	if !strings.Contains(out, `href="/changelog/"`) {
		t.Fatalf("changelog link not rewritten: %q", out)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestRewriteDocLinksUnknownWarns(t *testing.T) {
	out, warnings := rewriteDocLinks(`<a href="ghost.md">x</a>`, routeMap())
	if !strings.Contains(out, `href="ghost.md"`) {
		t.Fatalf("unknown link should be left as-is: %q", out)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "ghost.md") {
		t.Fatalf("unknown link should warn: %v", warnings)
	}
}
