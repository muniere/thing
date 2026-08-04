package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
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

func TestLoadFilterAllKeys(t *testing.T) {
	dir := write(t, `filter:
  statuses: [todo, doing]
  priorities: [high]
  category: Project
  tag: wip
  query: api
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f := cfg.Filter
	if f == nil {
		t.Fatal("Filter = nil, want a parsed block")
	}
	if len(f.Statuses) != 2 || f.Statuses[0] != model.Todo || f.Statuses[1] != model.Doing {
		t.Errorf("Statuses = %v, want [todo doing]", f.Statuses)
	}
	if len(f.Priorities) != 1 || f.Priorities[0] != model.High {
		t.Errorf("Priorities = %v, want [high]", f.Priorities)
	}
	if f.Category != "Project" || f.Tag != "wip" || f.Query != "api" {
		t.Errorf("category=%q tag=%q query=%q", f.Category, f.Tag, f.Query)
	}
	for _, k := range filterKeys {
		if !f.Has(k) {
			t.Errorf("Has(%q) = false, want true", k)
		}
	}
}

// A written-but-null key is present with an empty value: it clears an inherited
// filter instead of inheriting it.
func TestLoadFilterNullKeyIsPresent(t *testing.T) {
	dir := write(t, "filter:\n  tag:\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Filter.Has("tag") {
		t.Error(`Has("tag") = false, want true for an explicit null`)
	}
	if cfg.Filter.Tag != "" {
		t.Errorf("Tag = %q, want empty", cfg.Filter.Tag)
	}
	if cfg.Filter.Has("statuses") {
		t.Error(`Has("statuses") = true, want false for an omitted key`)
	}
}

// An empty value spells the same thing as null: present, and not filtering.
func TestLoadFilterEmptyValueIsPresent(t *testing.T) {
	dir := write(t, "filter:\n  statuses: []\n  category: \"\"\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Filter.Has("statuses") || len(cfg.Filter.Statuses) != 0 {
		t.Errorf("statuses: has=%v value=%v", cfg.Filter.Has("statuses"), cfg.Filter.Statuses)
	}
	if !cfg.Filter.Has("category") || cfg.Filter.Category != "" {
		t.Errorf("category: has=%v value=%q", cfg.Filter.Has("category"), cfg.Filter.Category)
	}
}

func TestLoadNoFilterBlock(t *testing.T) {
	dir := write(t, "title: Board\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Filter != nil {
		t.Errorf("Filter = %+v, want nil", cfg.Filter)
	}
	// A nil filter answers Has without panicking.
	if cfg.Filter.Has("statuses") {
		t.Error("nil Filter Has = true, want false")
	}
}

func TestLoadFilterInvalidValues(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"status", "filter:\n  statuses: [wip]\n", "wip"},
		{"priority", "filter:\n  priorities: [urgent]\n", "urgent"},
		{"key", "filter:\n  statues: [todo]\n", "statues"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil {
				t.Fatal("Load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// load is write+Load, for tests that only care about the resulting Config.
func load(t *testing.T, body string) *Config {
	t.Helper()
	cfg, err := Load(write(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestResolveFilterNeitherConfigured(t *testing.T) {
	if f := ResolveFilter(load(t, "title: A\n"), load(t, "title: B\n")); f != nil {
		t.Errorf("ResolveFilter = %+v, want nil", f)
	}
	if f := ResolveFilter(nil, nil); f != nil {
		t.Errorf("ResolveFilter(nil, nil) = %+v, want nil", f)
	}
}

func TestResolveFilterGlobalOnly(t *testing.T) {
	global := load(t, "filter:\n  statuses: [todo, doing]\n  tag: wip\n")
	f := ResolveFilter(global, load(t, "title: Project\n"))
	if f == nil {
		t.Fatal("ResolveFilter = nil, want the global filter")
	}
	if len(f.Statuses) != 2 || f.Tag != "wip" {
		t.Errorf("statuses=%v tag=%q, want [todo doing] and wip", f.Statuses, f.Tag)
	}
}

func TestResolveFilterProjectOnly(t *testing.T) {
	f := ResolveFilter(load(t, "title: Global\n"), load(t, "filter:\n  priorities: [high]\n"))
	if f == nil || len(f.Priorities) != 1 || f.Priorities[0] != model.High {
		t.Fatalf("ResolveFilter = %+v, want priorities [high]", f)
	}
}

// A key the project writes wins; keys it omits fall back to global.
func TestResolveFilterPerKeyOverride(t *testing.T) {
	global := load(t, "filter:\n  statuses: [todo, doing]\n  tag: wip\n")
	project := load(t, "filter:\n  tag: api\n")
	f := ResolveFilter(global, project)
	if f.Tag != "api" {
		t.Errorf("Tag = %q, want api (the project's value)", f.Tag)
	}
	if len(f.Statuses) != 2 {
		t.Errorf("Statuses = %v, want the inherited [todo doing]", f.Statuses)
	}
}

// An explicit null clears the inherited value rather than inheriting it.
func TestResolveFilterNullClears(t *testing.T) {
	global := load(t, "filter:\n  statuses: [todo]\n  tag: wip\n")
	project := load(t, "filter:\n  tag:\n")
	f := ResolveFilter(global, project)
	if f.Tag != "" {
		t.Errorf("Tag = %q, want empty (cleared)", f.Tag)
	}
	if len(f.Statuses) != 1 {
		t.Errorf("Statuses = %v, want the inherited [todo]", f.Statuses)
	}
}

// The resolved filter answers Has for the keys that ended up set, so a caller can
// tell a resolved-but-empty facet from an unset one.
func TestResolveFilterTracksPresence(t *testing.T) {
	f := ResolveFilter(load(t, "filter:\n  statuses: [todo]\n"), load(t, "filter:\n  tag:\n"))
	if !f.Has("statuses") || !f.Has("tag") {
		t.Errorf("Has: statuses=%v tag=%v, want both true", f.Has("statuses"), f.Has("tag"))
	}
	if f.Has("category") {
		t.Error(`Has("category") = true, want false`)
	}
}
