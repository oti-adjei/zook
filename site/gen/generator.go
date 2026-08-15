package main

import (
	"html/template"
	"io"
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

// RenderAll renders every page and copies static assets, returning link warnings.
func (g *Generator) RenderAll() ([]string, error) {
	if err := g.loadTemplates(); err != nil {
		return nil, err
	}
	var warnings []string
	routes := routeMap()

	// landing
	if err := g.renderPage("landing", "", pageData{Title: "zook — tiny deploy controller", Section: "landing", Nav: g.Nav}); err != nil {
		return warnings, err
	}

	// docs
	for _, n := range g.Nav {
		title, body, w, err := g.renderMarkdownFile(filepath.Join(g.HandbookDir, n.File), routes)
		if err != nil {
			return warnings, err
		}
		warnings = append(warnings, w...)
		if title == "" {
			title = n.Title
		}
		if err := g.renderPage("doc", "docs/"+n.Slug, pageData{
			Title: title, Section: "docs", Body: body, Nav: g.Nav, ActiveSlug: n.Slug,
		}); err != nil {
			return warnings, err
		}
	}

	// install
	title, body, w, err := g.renderMarkdownFile(filepath.Join(g.ContentDir, "install.md"), routes)
	if err != nil {
		return warnings, err
	}
	warnings = append(warnings, w...)
	if title == "" {
		title = "Install"
	}
	if err := g.renderPage("page", "install", pageData{Title: title, Section: "page", Body: body, Nav: g.Nav}); err != nil {
		return warnings, err
	}

	// changelog
	title, body, w, err = g.renderMarkdownFile(g.ChangelogPath, routes)
	if err != nil {
		return warnings, err
	}
	warnings = append(warnings, w...)
	if title == "" {
		title = "Changelog"
	}
	if err := g.renderPage("page", "changelog", pageData{Title: title, Section: "page", Body: body, Nav: g.Nav}); err != nil {
		return warnings, err
	}

	// static
	if err := g.copyStatic(); err != nil {
		return warnings, err
	}
	return warnings, nil
}

// renderMarkdownFile reads a markdown file, extracts its title, renders it, and
// rewrites inter-page links.
func (g *Generator) renderMarkdownFile(srcPath string, routes map[string]string) (string, template.HTML, []string, error) {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return "", "", nil, err
	}
	title, mdBody := extractTitle(src)
	rendered, err := renderMarkdown(mdBody)
	if err != nil {
		return "", "", nil, err
	}
	rewritten, warnings := rewriteDocLinks(rendered, routes)
	return title, template.HTML(rewritten), warnings, nil
}

// copyStatic mirrors StaticDir into OutDir/static.
func (g *Generator) copyStatic() error {
	return filepath.Walk(g.StaticDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(g.StaticDir, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(g.OutDir, "static", rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
