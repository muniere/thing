package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
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
	if len(got.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(got.Projects))
	}
	if got.Projects[0].Name != "work" || got.Projects[0].Dir != "/Users/me/work/.thing" {
		t.Errorf("project[0] = %+v", got.Projects[0])
	}
	if got.Projects[1].Name != "home" || got.Projects[1].Dir != "/Users/me/home/.thing" {
		t.Errorf("project[1] = %+v", got.Projects[1])
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	// A missing projects.yaml is not an error: the server starts with no projects.
	got, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(got.Projects) != 0 {
		t.Errorf("got %d projects, want 0", len(got.Projects))
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

func TestSaveRoundTrips(t *testing.T) {
	// Save into a not-yet-existing state dir, then Load it back unchanged.
	path := filepath.Join(t.TempDir(), "state", "projects.yaml")
	want := []Project{
		{Name: "work", Dir: "/Users/me/work/.thing"},
		{Name: "home", Dir: "/Users/me/home/.thing"},
	}
	if err := Save(path, &Registry{Projects: want}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Projects) != len(want) {
		t.Fatalf("got %d projects, want %d", len(got.Projects), len(want))
	}
	for i := range want {
		if got.Projects[i] != want[i] {
			t.Errorf("project[%d] = %+v, want %+v", i, got.Projects[i], want[i])
		}
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	// A second Save replaces the file rather than appending; the empty list
	// yields an empty registry.
	path := filepath.Join(t.TempDir(), "projects.yaml")
	if err := Save(path, &Registry{Projects: []Project{{Name: "work", Dir: "/a/.thing"}}}); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	if err := Save(path, &Registry{}); err != nil {
		t.Fatalf("Save empty: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Projects) != 0 {
		t.Errorf("got %d projects, want 0", len(got.Projects))
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	// A bad list is rejected before it reaches disk, so no file is created.
	path := filepath.Join(t.TempDir(), "projects.yaml")
	if err := Save(path, &Registry{Projects: []Project{{Name: "Not A Slug", Dir: "/a/.thing"}}}); err == nil {
		t.Error("expected an error for a non-slug project name")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no file written on invalid input, stat err = %v", err)
	}
}

// writeReg drops projects.yaml holding body into a temp dir and returns its path.
func writeReg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A project's own filter block is read from its entry, and the top-level
// defaults apply to every project that does not write the key itself.
func TestLoadFilters(t *testing.T) {
	reg, err := Load(writeReg(t, `defaults:
  filter:
    statuses: [todo, doing]
    tag: wip
projects:
  - name: work
    dir: /tmp/work
    filter:
      tag: api
  - name: home
    dir: /tmp/home
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// work overrides tag and inherits statuses.
	work := reg.Filters("work")
	if work.Tag != "api" {
		t.Errorf("work tag = %q, want api", work.Tag)
	}
	if len(work.Statuses) != 2 || work.Statuses[0] != model.Todo {
		t.Errorf("work statuses = %v, want the inherited [todo doing]", work.Statuses)
	}

	// home writes nothing, so it is the defaults.
	home := reg.Filters("home")
	if home.Tag != "wip" || len(home.Statuses) != 2 {
		t.Errorf("home = %+v, want the defaults", home)
	}
}

// A project registered at runtime has no entry filter yet, so it starts from the
// defaults like any other.
func TestFiltersUnknownProjectGetsDefaults(t *testing.T) {
	reg, err := Load(writeReg(t, "defaults:\n  filter:\n    tag: wip\nprojects: []\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f := reg.Filters("new"); f == nil || f.Tag != "wip" {
		t.Errorf("Filters(new) = %+v, want the defaults", f)
	}
}

// An explicit null clears an inherited value instead of inheriting it.
func TestFiltersNullClears(t *testing.T) {
	reg, err := Load(writeReg(t, `defaults:
  filter:
    statuses: [todo]
    tag: wip
projects:
  - name: work
    dir: /tmp/work
    filter:
      tag:
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f := reg.Filters("work")
	if f.Tag != "" {
		t.Errorf("tag = %q, want it cleared", f.Tag)
	}
	if !f.Has("tag") {
		t.Error(`Has("tag") = false, want true — a cleared key is still set`)
	}
	if len(f.Statuses) != 1 {
		t.Errorf("statuses = %v, want the inherited [todo]", f.Statuses)
	}
}

// With nothing configured anywhere there is no filter at all, so the board is
// unfiltered rather than filtered by empty facets.
func TestFiltersNoneConfigured(t *testing.T) {
	reg, err := Load(writeReg(t, "projects:\n  - name: work\n    dir: /tmp/work\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f := reg.Filters("work"); f != nil {
		t.Errorf("Filters = %+v, want nil", f)
	}
}

func TestLoadFilterInvalid(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"status", "projects:\n  - name: a\n    dir: /tmp/a\n    filter:\n      statuses: [wip]\n", "wip"},
		{"priority", "defaults:\n  filter:\n    priorities: [urgent]\n", "urgent"},
		{"key", "projects:\n  - name: a\n    dir: /tmp/a\n    filter:\n      statues: [todo]\n", "statues"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(writeReg(t, tc.body)); err == nil {
				t.Fatal("Load succeeded, want an error")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// Registering or unregistering a project rewrites the whole file, so everything
// it did not touch — the defaults, and other projects' filters — has to survive
// the round trip. An omitted key must not come back as an empty one either: that
// would silently turn "inherit" into "clear".
func TestSavePreservesFilters(t *testing.T) {
	path := writeReg(t, `defaults:
  filter:
    statuses: [todo, doing]
projects:
  - name: work
    dir: /tmp/work
    filter:
      tag: api
`)
	reg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	reg.Projects = append(reg.Projects, Project{Name: "extra", Dir: "/tmp/extra"})
	if err := Save(path, reg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	back, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if f := back.Filters("work"); f.Tag != "api" || len(f.Statuses) != 2 {
		t.Errorf("work filter after round trip = %+v, want tag api and the inherited statuses", f)
	}
	if f := back.Filters("extra"); f.Has("tag") {
		t.Errorf("extra inherited a written tag key: %+v", f)
	}
	if len(back.Projects) != 2 {
		t.Errorf("projects = %d, want 2", len(back.Projects))
	}
}

// A project's own theme wins over the registry-wide default; either alone stands
// on its own.
func TestResolveTheme(t *testing.T) {
	for _, tc := range []struct{ name, defaults, project, want string }{
		{"project wins", "amber", "teal", "teal"},
		{"default fallback", "amber", "", "amber"},
		{"project only", "", "violet", "violet"},
		{"neither", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveTheme(tc.defaults, tc.project); got != tc.want {
				t.Errorf("ResolveTheme(%q, %q) = %q, want %q", tc.defaults, tc.project, got, tc.want)
			}
		})
	}
}

// Which themes exist is a question about files, not about this package, so a name
// is only checked for being safe to put in an attribute and a URL path.
func TestResolveThemeRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{`"><script>`, "Teal", "te al", "teal;", "../teal", "-teal"} {
		if got := ResolveTheme("", name); got != "" {
			t.Errorf("ResolveTheme(%q) = %q, want empty", name, got)
		}
	}
}

// The theme reads from the same two places the filter does, so one entry
// configures a board completely.
func TestLoadThemes(t *testing.T) {
	reg, err := Load(writeReg(t, `defaults:
  theme: slate
projects:
  - name: work
    dir: /tmp/work
    theme: teal
  - name: home
    dir: /tmp/home
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := ResolveTheme(reg.Defaults.Theme, reg.Projects[0].Theme); got != "teal" {
		t.Errorf("work theme = %q, want teal", got)
	}
	if got := ResolveTheme(reg.Defaults.Theme, reg.Projects[1].Theme); got != "slate" {
		t.Errorf("home theme = %q, want the default slate", got)
	}
}
