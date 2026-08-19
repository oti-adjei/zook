package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

// version is stamped at build time via -ldflags "-X ...cli.version=<tag>";
// GoReleaser injects the git tag. When unset it stays "dev" and printVersion
// falls back to the build info.
var version = "dev"

// printVersion writes zook's version. A release build stamps the tag into
// `version`; otherwise it is derived from the build's module version and VCS
// revision (as with `go install`), falling back to "dev".
func printVersion(w io.Writer) {
	if version != "dev" {
		fmt.Fprintf(w, "zook %s\n", version)
		return
	}
	v := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		var rev string
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
		}
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			v = fmt.Sprintf("%s (%s)", v, rev)
		}
	}
	fmt.Fprintf(w, "zook %s\n", v)
}
