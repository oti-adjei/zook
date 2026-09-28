package main

// NavItem is one entry in the docs sidebar. File is the source markdown file
// under handbook/; Slug is its URL segment (/docs/<Slug>/); Title is the label.
type NavItem struct {
	Slug  string
	Title string
	File  string
}

// docsNav is the ordered docs navigation. Adding a handbook page means adding
// one entry here.
var docsNav = []NavItem{
	{Slug: "overview", Title: "Overview", File: "README.md"},
	{Slug: "concepts", Title: "Concepts", File: "concepts.md"},
	{Slug: "layout", Title: "Layout", File: "layout.md"},
	{Slug: "deploy-contract", Title: "Deploy Contract", File: "deploy-contract.md"},
	{Slug: "commands", Title: "Commands", File: "commands.md"},
	{Slug: "stacks", Title: "Stacks", File: "stacks.md"},
	{Slug: "architecture", Title: "Architecture", File: "architecture.md"},
	{Slug: "native", Title: "Native Runtime", File: "native.md"},
	{Slug: "serve", Title: "Serve (HTTP API)", File: "serve.md"},
	{Slug: "roadmap", Title: "Roadmap", File: "roadmap.md"},
}

// routeMap maps a source markdown basename to its destination site route, used
// to rewrite the handbook's inter-page .md links.
func routeMap() map[string]string {
	m := map[string]string{"CHANGELOG.md": "/changelog/"}
	for _, n := range docsNav {
		m[n.File] = "/docs/" + n.Slug + "/"
	}
	return m
}
