package cli

import (
	"fmt"
	"io"
	"runtime/debug"
)

// printVersion writes zook's version, derived from the build's module version
// and VCS revision when available (falls back to "dev").
func printVersion(w io.Writer) {
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
