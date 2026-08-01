// Command thing is a CLI for a topic tree of Epic > Issue > Task nodes.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/muniere/thing/internal/cli"
)

// version is the CLI version, overridable at build time via -ldflags.
var version = "1.3.0"

func main() {
	err := cli.NewRootCmd(version).Execute()
	// A command may request a specific exit code with no message (it already
	// wrote its own output) — e.g. `server status` printing "stopped" then
	// exiting non-zero.
	if exit, ok := errors.AsType[*cli.ExitError](err); ok {
		os.Exit(exit.Code)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "thing:", err)
		os.Exit(1)
	}
}
