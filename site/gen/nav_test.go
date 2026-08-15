package main

import "testing"

func TestDocsNavCoversHandbook(t *testing.T) {
	// Every handbook page we ship must have a nav entry.
	wantFiles := []string{
		"README.md", "concepts.md", "layout.md", "deploy-contract.md",
		"commands.md", "stacks.md", "architecture.md", "native.md", "roadmap.md",
	}
	have := map[string]bool{}
	for _, n := range docsNav {
		have[n.File] = true
		if n.Slug == "" || n.Title == "" {
			t.Fatalf("nav item for %q missing slug/title", n.File)
		}
	}
	for _, f := range wantFiles {
		if !have[f] {
			t.Fatalf("docsNav missing handbook file %q", f)
		}
	}
}

func TestRouteMap(t *testing.T) {
	m := routeMap()
	if m["concepts.md"] != "/docs/concepts/" {
		t.Fatalf("concepts.md → %q, want /docs/concepts/", m["concepts.md"])
	}
	if m["README.md"] != "/docs/overview/" {
		t.Fatalf("README.md → %q, want /docs/overview/", m["README.md"])
	}
	if m["CHANGELOG.md"] != "/changelog/" {
		t.Fatalf("CHANGELOG.md → %q, want /changelog/", m["CHANGELOG.md"])
	}
}
