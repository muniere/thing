package server

import (
	"fmt"
	"io"
	"os"
)

// ANSI escape codes for the startup banner; only emitted to a terminal.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiAmber  = "\x1b[33m"
	ansiGreen  = "\x1b[32m"
	ansiCyanUL = "\x1b[36;4m" // cyan + underline, for the URL
)

// PrintStartup writes a Next.js-style startup banner so it stands out from the
// access-log lines that follow, matching the tray web server's. Colors are used
// only when out is a terminal.
func PrintStartup(out io.Writer, version, url, dir string) {
	c := func(code, s string) string {
		if !isTerminal(out) {
			return s
		}
		return code + s + ansiReset
	}

	fmt.Fprintf(out, "\n  %s %s %s\n\n",
		c(ansiAmber, "▲"), c(ansiBold, "thing"), c(ansiDim, version))
	fmt.Fprintf(out, "  %s   %s\n", c(ansiDim, "Local:"), c(ansiCyanUL, url))
	fmt.Fprintf(out, "  %s     %s\n", c(ansiDim, "Dir:"), dir)
	fmt.Fprintf(out, "\n  %s Ready — live reload on, Ctrl+C to stop\n\n",
		c(ansiGreen, "✓"))
}

// isTerminal reports whether w is a character device (a TTY).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
