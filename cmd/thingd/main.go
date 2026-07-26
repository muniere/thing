// Command thingd is the human-facing web server for thing trees. One process
// hosts multiple projects registered in projects.yaml, each a named mount over
// its own data directory, addressed under /<project>. It serves a JSON API over
// the shared Go data layer and the bundled SPA, which it embeds, so `make build`
// yields one self-contained binary and `make serve` runs that same binary under
// air.
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
	"github.com/muniere/thing/internal/registry"
	"github.com/muniere/thing/internal/server"
	"github.com/muniere/thing/internal/store"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "1.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("thingd", flag.ContinueOnError)
	fs.SetOutput(stderr)
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

	// Projects come from projects.yaml under the resolved state directory. A
	// missing file is not an error — the server starts empty and shows the picker.
	regFile, err := registry.File()
	if err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}
	projects, err := registry.Load(regFile)
	if err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}

	mounts := make([]server.Mount, 0, len(projects))
	infos := make([]server.ProjectInfo, 0, len(projects))
	for _, p := range projects {
		mounts = append(mounts, server.Mount{Name: p.Name, Store: store.Open(p.Dir)})
		infos = append(infos, server.ProjectInfo{Name: p.Name, Dir: p.Dir})
	}

	// An explicit --port must be honored exactly; the default may hop to the
	// next free port so multiple servers can run at once.
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

	srv := server.New(mounts, server.Options{
		Static: thing.WebAssets(),
		Now:    func() string { return time.Now().Format("2006-01-02") },
		Logger: log.New(stderr, "", log.LstdFlags),
	})

	// Watch every project's data dir so edits from the CLI or an editor live-reload
	// open browsers over SSE. The watchers run for the lifetime of the process.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.StartWatch(ctx, time.Second)

	// Show localhost (the bind host) with the actually-bound port, which may have
	// hopped past a taken default.
	url := fmt.Sprintf("http://localhost:%d", ln.Addr().(*net.TCPAddr).Port)
	server.PrintStartup(stdout, version, url, infos)
	if *open {
		openBrowser(url)
	}

	if err := http.Serve(ln, srv); err != nil {
		fmt.Fprintf(stderr, "thingd: %v\n", err)
		return 1
	}
	return 0
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
