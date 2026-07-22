package thing

import (
	"embed"
	"io/fs"
)

// webDist holds the built SPA. thingd embeds web/dist and serves the whole app
// from one self-contained binary, so `make build` produces a single artifact and
// dev (`make serve`) runs that same binary under air. The embed is
// unconditional, so web/dist must exist for any go build/test/vet — a committed
// web/dist/.gitkeep keeps it compiling before the first `make build`.
//
// `all:` overrides go:embed's default of skipping names that begin with "." or
// "_", so nothing a bundler emits (and the .gitkeep) can be silently dropped.
//
//go:embed all:web/dist
var webDist embed.FS

// WebAssets returns the embedded bundle rooted at index.html.
func WebAssets() fs.FS {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		panic(err) // the embed path is a build-time constant
	}
	return sub
}
