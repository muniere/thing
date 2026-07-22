//go:build modular

package thing

import "io/fs"

// WebAssets returns nil under -tags modular: the frontend and API run as
// separate pieces — thingd serves only the API (no embedded SPA, so it needs no
// built web/dist and go test / go vet stay frontend-free) while `vite dev` serves
// the UI. The default build bundles them into one binary instead — see assets.go.
func WebAssets() fs.FS { return nil }
