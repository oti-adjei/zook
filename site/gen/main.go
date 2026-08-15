package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	out := flag.String("out", "site/dist", "output directory")
	serve := flag.Bool("serve", false, "serve the output after building")
	addr := flag.String("addr", ":8080", "address for -serve")
	flag.Parse()

	g := &Generator{
		HandbookDir:   "handbook",
		ContentDir:    "site/content",
		TemplateDir:   "site/templates",
		StaticDir:     "site/static",
		ChangelogPath: "CHANGELOG.md",
		OutDir:        *out,
		Nav:           docsNav,
	}
	warnings, err := g.RenderAll()
	if err != nil {
		log.Fatalf("build failed: %v", err)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	fmt.Printf("built site → %s\n", *out)

	if *serve {
		fmt.Printf("serving %s on %s\n", *out, *addr)
		log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*out))))
	}
}
