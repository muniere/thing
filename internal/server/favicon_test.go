package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A tree carrying icon.svg has its own board icon, served as the file it is.
func TestIconEndpointCustomSVG(t *testing.T) {
	s := newServer(t)
	root := proj(t, s).store.Root
	body := `<svg xmlns="http://www.w3.org/2000/svg"><rect width="16" height="16"/></svg>`
	if err := os.WriteFile(filepath.Join(root, "icon.svg"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	w := do(t, s, "GET", "/api/projects/test/icon", "")
	if w.Code != http.StatusOK {
		t.Fatalf("icon = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	if w.Body.String() != body {
		t.Errorf("body = %q, want the file's contents", w.Body.String())
	}
}

// A PNG is served with its own type rather than sniffed into something else.
func TestIconEndpointCustomPNG(t *testing.T) {
	s := newServer(t)
	root := proj(t, s).store.Root
	// A real 1x1 PNG, so nothing along the way has to guess at the bytes.
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
		0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
		0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(filepath.Join(root, "icon.png"), png, 0o644); err != nil {
		t.Fatal(err)
	}
	w := do(t, s, "GET", "/api/projects/test/icon", "")
	if w.Code != http.StatusOK {
		t.Fatalf("icon = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if w.Body.Len() != len(png) {
		t.Errorf("body = %d bytes, want %d", w.Body.Len(), len(png))
	}
}

// A tree with no icon of its own still answers: the endpoint is what every
// board's <link rel="icon"> points at, so a 404 here would leave the tab with a
// blank icon rather than the board's mark.
func TestIconEndpointFallback(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "GET", "/api/projects/test/icon", "")
	if w.Code != http.StatusOK {
		t.Fatalf("icon = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	if !strings.Contains(w.Body.String(), "◉") {
		t.Errorf("body = %q, want the built-in mark", w.Body.String())
	}
}

// An unmounted project has no icon to serve.
func TestIconEndpointUnknownProject(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "GET", "/api/projects/nope/icon", ""); w.Code != http.StatusNotFound {
		t.Errorf("icon = %d, want 404", w.Code)
	}
}
