package main

import (
	"bytes"
	"fmt"
	"path"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var gm = goldmark.New(goldmark.WithExtensions(extension.GFM))

// h1Re matches the first ATX H1 line.
var h1Re = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

// extractTitle returns the first H1's text and the source with that line removed.
func extractTitle(src []byte) (string, []byte) {
	loc := h1Re.FindSubmatchIndex(src)
	if loc == nil {
		return "", src
	}
	title := string(src[loc[2]:loc[3]])
	// remove the whole matched line (loc[0]:loc[1]) plus a trailing newline if present
	end := loc[1]
	if end < len(src) && src[end] == '\n' {
		end++
	}
	body := append(append([]byte{}, src[:loc[0]]...), src[end:]...)
	return title, body
}

// renderMarkdown converts markdown (GFM) to an HTML fragment.
func renderMarkdown(src []byte) (string, error) {
	var buf bytes.Buffer
	if err := gm.Convert(src, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// mdLinkRe matches href="something.md" with an optional #anchor.
var mdLinkRe = regexp.MustCompile(`href="([^"]+?\.md)(#[^"]*)?"`)

// rewriteDocLinks rewrites inter-page .md links to their site routes. Unknown
// .md links are left unchanged and returned as warnings.
func rewriteDocLinks(htmlBody string, routes map[string]string) (string, []string) {
	var warnings []string
	out := mdLinkRe.ReplaceAllStringFunc(htmlBody, func(m string) string {
		sub := mdLinkRe.FindStringSubmatch(m)
		base := path.Base(sub[1])
		anchor := sub[2]
		if route, ok := routes[base]; ok {
			return fmt.Sprintf(`href="%s%s"`, route, anchor)
		}
		warnings = append(warnings, fmt.Sprintf("unresolved link: %s", sub[1]))
		return m
	})
	return out, warnings
}
