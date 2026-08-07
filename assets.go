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

// themes holds the built-in theme stylesheets. They are embedded separately from
// the SPA bundle because they are not bundled at all: thingd serves each one
// as-is when the board asks for it, so that the same route can also serve themes
// the reader drops in a directory on disk.
//
//go:embed web/themes/*.css
var themes embed.FS

// ThemeAssets returns the built-in themes rooted at the directory holding them,
// so a theme is addressed as "<name>.css".
func ThemeAssets() fs.FS {
	sub, err := fs.Sub(themes, "web/themes")
	if err != nil {
		panic(err) // the embed path is a build-time constant
	}
	return sub
}
