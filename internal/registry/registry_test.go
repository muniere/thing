package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateDir(t *testing.T) {
	// THING_DATA_DIR wins and is used verbatim (projects.yaml sits directly in it).
	t.Setenv("THING_DATA_DIR", "/data/thingd")
	t.Setenv("XDG_STATE_HOME", "/xdg/state")
	if got, _ := StateDir(); got != "/data/thingd" {
		t.Errorf("THING_DATA_DIR: got %q, want /data/thingd", got)
	}

	// Without THING_DATA_DIR, an absolute XDG_STATE_HOME yields $XDG_STATE_HOME/thingd.
	t.Setenv("THING_DATA_DIR", "")
	if got, _ := StateDir(); got != filepath.Join("/xdg/state", "thingd") {
		t.Errorf("XDG_STATE_HOME: got %q, want /xdg/state/thingd", got)
	}

	// A relative XDG_STATE_HOME is ignored (XDG spec), falling back to ~/.local/state/thingd.
	t.Setenv("XDG_STATE_HOME", "relative/path")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".local", "state", "thingd")
	if got, _ := StateDir(); got != want {
		t.Errorf("fallback: got %q, want %q", got, want)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.yaml")
	if err := os.WriteFile(path, []byte(`projects:
  - name: work
    dir: /Users/me/work/.thing
  - name: home
    dir: /Users/me/home/.thing
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2", len(got))
	}
	if got[0].Name != "work" || got[0].Dir != "/Users/me/work/.thing" {
		t.Errorf("project[0] = %+v", got[0])
	}
	if got[1].Name != "home" || got[1].Dir != "/Users/me/home/.thing" {
		t.Errorf("project[1] = %+v", got[1])
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	// A missing projects.yaml is not an error: the server starts with no projects.
	got, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d projects, want 0", len(got))
	}
}

func TestLoadRejectsDuplicateName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.yaml")
	if err := os.WriteFile(path, []byte(`projects:
  - name: work
    dir: /a/.thing
  - name: work
    dir: /b/.thing
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected an error for a duplicate project name")
	}
}

func TestLoadRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.yaml")
	if err := os.WriteFile(path, []byte(`projects:
  - name: Not A Slug
    dir: /a/.thing
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected an error for a non-slug project name")
	}
}
