package main

import (
	"html/template"
	"os"
	"path/filepath"
)

// pageData is the data passed to every template render.
type pageData struct {
	Title      string
	Section    string
	Body       template.HTML
	Nav        []NavItem
	ActiveSlug string
}

// Generator renders the static site from markdown sources into OutDir.
type Generator struct {
	HandbookDir   string
	ContentDir    string
	TemplateDir   string
	StaticDir     string
	ChangelogPath string
	OutDir        string
	Nav           []NavItem
	templates     map[string]*template.Template
}

// loadTemplates parses base.html.tmpl with each page template into a named set.
func (g *Generator) loadTemplates() error {
	base := filepath.Join(g.TemplateDir, "base.html.tmpl")
	sets := map[string]string{
		"landing": "landing.html.tmpl",
		"doc":     "doc.html.tmpl",
		"page":    "page.html.tmpl",
	}
	g.templates = map[string]*template.Template{}
	for name, file := range sets {
		t, err := template.ParseFiles(base, filepath.Join(g.TemplateDir, file))
		if err != nil {
			return err
		}
		g.templates[name] = t
	}
	return nil
}

// renderPage executes the named template set's base into <OutDir>/<outRoute>/index.html.
// An empty outRoute writes <OutDir>/index.html.
func (g *Generator) renderPage(setName, outRoute string, data pageData) error {
	t, ok := g.templates[setName]
	if !ok {
		return os.ErrInvalid
	}
	dir := g.OutDir
	if outRoute != "" {
		dir = filepath.Join(g.OutDir, filepath.FromSlash(outRoute))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	return t.ExecuteTemplate(f, "base", data)
}
