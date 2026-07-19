// Command thing is a CLI for a topic tree of Epic > Issue > Task nodes.
//
// This scaffold build only prints usage; the commands are wired up in later
// commits.
package main

import "fmt"

const usage = `thing — manage a topic tree of Epic > Issue > Task nodes.

Usage:
  thing <command> [arguments]

No commands are available yet; this is the project scaffold.
`

func main() {
	fmt.Print(usage)
}
