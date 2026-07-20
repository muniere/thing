// Command thing is a CLI for a topic tree of Epic > Issue > Task nodes.
package main

import (
	"fmt"
	"os"

	"github.com/muniere/thing/internal/cli"
)

// version is the CLI version, overridable at build time via -ldflags.
var version = "0.0.1-dev"

func main() {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "thing:", err)
		os.Exit(1)
	}
}
