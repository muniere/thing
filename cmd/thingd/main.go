// Command thingd is the human-facing web server for a thing tree. It serves a
// JSON API over the shared Go data layer and the bundled SPA, which it embeds, so
// `make build` yields one self-contained binary and `make serve` runs that same
// binary under air.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	thing "github.com/muniere/thing"
	"github.com/muniere/thing/internal/server"
	"github.com/muniere/thing/internal/store"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("thingd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "data directory")
	global := fs.Bool("global", false, "use the global tree (~/.thing)")
	fs.BoolVar(global, "g", false, "shorthand for --global")
	port := fs.Int("port", server.DefaultPort, "listen port")
	open := fs.Bool("open", false, "open the URL in a browser once serving")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	root, err := resolveDataDir(*dir, *global)
	if err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}

	// An explicit --port must be honored exactly; the default may hop to the
	// next free port so multiple trees can serve at once.
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			explicit = true
		}
	})
	ln, err := server.Listen(*port, explicit)
	if err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}

	st := store.Open(root)
	srv := server.New(st, server.Options{
		Static: thing.WebAssets(),
		Now:    func() string { return time.Now().Format("2006-01-02") },
		Logger: log.New(stderr, "", log.LstdFlags),
	})

	// Watch the data dir so edits from the CLI or an editor live-reload open
	// browsers over SSE. The watcher runs for the lifetime of the process.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.StartWatch(ctx, time.Second)

	// Show localhost (the bind host) with the actually-bound port, which may have
	// hopped past a taken default.
	url := fmt.Sprintf("http://localhost:%d", ln.Addr().(*net.TCPAddr).Port)
	server.PrintStartup(stdout, version, url, root)
	if *open {
		openBrowser(url)
	}

	if err := http.Serve(ln, srv); err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}
	return 0
}

// resolveDataDir picks the tree's data directory, mirroring the CLI's order:
//
//	--dir  ->  THING_DATA_DIR  ->  -g global  ->  nearest .thing/ upward
//
// Like the CLI there is no implicit global fallback: it errors rather than
// silently serve a global tree.
func resolveDataDir(dir string, global bool) (string, error) {
	if dir != "" {
		return dir, nil
	}
	if v := os.Getenv("THING_DATA_DIR"); v != "" {
		return v, nil
	}
	if global {
		return store.GlobalDataDir()
	}
	if d, ok := store.FindProjectDir(); ok {
		return d, nil
	}
	return "", fmt.Errorf("no data directory found: pass --dir, set THING_DATA_DIR, use -g for the global tree, or run inside a project (a directory with a .thing/)")
}

// openBrowser best-effort launches the platform's URL opener. Failures are
// silent: the URL is already printed for the user to click.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, url)...).Start()
}
