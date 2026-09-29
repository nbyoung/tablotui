// Command tablotui is the Tableaux terminal front end.
//
// This bootstrap prints the version and exits. The full-screen program comes
// in later tasks.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version is the release version. GoReleaser sets it through
// -ldflags "-X main.version=...". A build from source leaves it at "dev".
var version = "dev"

func main() {
	info, _ := debug.ReadBuildInfo()
	fmt.Fprintln(os.Stdout, banner(resolveVersion(version, info)))
}

// resolveVersion picks the version to report. A version set by the linker
// wins; otherwise the module version that go install recorded; otherwise
// "dev".
func resolveVersion(linked string, info *debug.BuildInfo) string {
	if linked != "" && linked != "dev" {
		return linked
	}
	if info != nil && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// banner is the one line the command prints.
func banner(v string) string {
	return "tablotui " + v
}
