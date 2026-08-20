package config

import (
	"os"
	"path/filepath"
	"testing"
)

// write drops a config.yaml holding body into a fresh directory and returns it.
func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoad(t *testing.T) {
	cfg, err := Load(write(t, "title: My Board\ncategories:\n  - Project\n  - Personal\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Title != "My Board" {
		t.Errorf("Title = %q, want My Board", cfg.Title)
	}
	if len(cfg.Categories) != 2 || cfg.Categories[0] != "Project" {
		t.Errorf("Categories = %v, want [Project Personal]", cfg.Categories)
	}
}

// A directory with no config.yaml is the common case for a fresh tree, so it
// reads as an empty config rather than an error.
func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Title != "" || len(cfg.Categories) != 0 {
		t.Errorf("Load = %+v, want the zero config", cfg)
	}
}

// The board's starting filter belongs to thingd, not to the tree, so it lives on
// the project's projects.yaml entry. A leftover `filter` block here is ignored
// rather than rejected: the CLI reads this file too, and a stale key must not
// stop `thing tree` from running.
func TestLoadIgnoresServerSettings(t *testing.T) {
	cfg, err := Load(write(t, "title: Board\nfilter:\n  statuses: [todo]\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Title != "Board" {
		t.Errorf("Title = %q, want Board", cfg.Title)
	}
}

// A filter block left in config.yaml after the move to projects.yaml is reported
// rather than silently ignored.
func TestStaleServerKeys(t *testing.T) {
	if got := StaleServerKeys(write(t, "title: Board\nfilter:\n  statuses: [todo]\n")); len(got) != 1 || got[0] != "filter" {
		t.Errorf("StaleServerKeys = %v, want [filter]", got)
	}
	if got := StaleServerKeys(write(t, "title: Board\n")); len(got) != 0 {
		t.Errorf("StaleServerKeys = %v, want none", got)
	}
	if got := StaleServerKeys(t.TempDir()); len(got) != 0 {
		t.Errorf("StaleServerKeys on a dir with no config = %v, want none", got)
	}
}

// A tree may carry its own board icon next to config.yaml, so Icon reports the
// conventional file it finds.
func TestIcon(t *testing.T) {
	dir := write(t, "title: Board\n")
	if err := os.WriteFile(filepath.Join(dir, "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, ok := Icon(dir)
	if !ok || name != "icon.png" {
		t.Errorf("Icon = %q, %v; want icon.png, true", name, ok)
	}
}

// SVG wins over PNG: it is the one that stays sharp at every size a tab or a
// dock asks for, so a tree carrying both is taken to prefer it.
func TestIconPrefersSVG(t *testing.T) {
	dir := write(t, "title: Board\n")
	for _, name := range []string{"icon.png", "icon.svg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if name, ok := Icon(dir); !ok || name != "icon.svg" {
		t.Errorf("Icon = %q, %v; want icon.svg, true", name, ok)
	}
}

// No icon is the common case, and it is not an error: the board falls back to
// its built-in mark.
func TestIconMissing(t *testing.T) {
	if name, ok := Icon(t.TempDir()); ok || name != "" {
		t.Errorf("Icon = %q, %v; want \"\", false", name, ok)
	}
}
