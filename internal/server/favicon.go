package server

// Board icons. A board's tab takes its icon from this one endpoint, which serves
// the icon the tree carries next to its config.yaml (icon.svg or icon.png) and
// falls back to thing's own mark. A project's icon is part of what the tree says
// about itself, like its title, so it is found by convention in the data
// directory rather than configured on the server side.
//
// It answers under /api/projects/<project>/ rather than under the board's own
// path because a route with a wildcard first segment ("/{project}/icon.svg")
// overlaps the existing "/themes/{file}" without being more specific than it,
// which ServeMux rejects as a conflict.
//
// The endpoint always answers 200. Every board points its <link rel="icon"> at
// the same URL and the frontend cannot know which trees carry an icon, so a
// project without one still has to be served something: a 404 would leave the
// tab with a blank icon rather than fall back to the mark.

import (
	"net/http"
	"path/filepath"

	"github.com/muniere/thing/internal/config"
)

// defaultIcon is the mark a board wears when its tree carries no icon of its
// own — the same ◉ the shell ships in web/index.html, so the fallback and the
// picker's icon are one drawing rather than two that nearly match.
const defaultIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><text y="13" font-size="13">◉</text></svg>`

// iconTypes maps the extensions config.Icon can return to the media type each is
// served as. The table is here rather than in mime.TypeByExtension because that
// consults the host's mime.types, which can leave .svg unregistered on a machine
// thingd has no say over.
var iconTypes = map[string]string{".svg": "image/svg+xml", ".png": "image/png"}

// handleIcon serves the project's icon, or the built-in mark when its tree
// carries none.
func (s *Server) handleIcon(p *project, w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	root := p.store.Root
	p.mu.RUnlock()
	if name, ok := config.Icon(root); ok {
		// Set before ServeFile, which honors a Content-Type already on the response
		// and so never sniffs the file. ServeFile also brings Last-Modified and the
		// 304 that follows from it, which is why the icon is handed over as a file:
		// an icon replaced in the data directory reaches the tab on the next reload
		// without the server tracking anything.
		w.Header().Set("Content-Type", iconTypes[filepath.Ext(name)])
		http.ServeFile(w, r, filepath.Join(root, name))
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	if _, err := w.Write([]byte(defaultIcon)); err != nil && s.logger != nil {
		s.logger.Printf("write /api/projects/%s/icon: %v", p.name, err)
	}
}
