//go:build !modular

package thing

import (
	"embed"
	"io/fs"
)

// webDist holds the built SPA. This is the default (release) build: thingd embeds
// web/dist and serves the whole app from one self-contained binary, as
// `make build` produces. Build with -tags modular to embed nothing and run the
// frontend and API as separate pieces (so it needs no built web/dist and
// `vite dev` serves the frontend) — see assets_modular.go.
//
// `all:` overrides go:embed's default of skipping names that begin with "." or
// "_", so nothing a bundler emits can be silently dropped from the bundle.
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
