// Package registry resolves and loads thingd's project registry: the mapping of
// URL-safe project names to their data directories. The registry is server-level
// state (thingd writes it back on dynamic register/unregister), so it lives under
// the XDG state directory rather than config.
package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/muniere/thing/internal/slug"
	"gopkg.in/yaml.v3"
)

// FileName is the registry file's name within the state directory.
const FileName = "projects.yaml"

// Project is one registered project: a URL-safe name, its data directory, and
// the display settings thingd applies to that board. Only settings thingd itself
// owns live here — what a tree *is* (its title and categories) stays in the
// tree's own config.yaml, which the CLI reads too.
type Project struct {
	Name   string  `yaml:"name"`
	Dir    string  `yaml:"dir"`
	Filter *Filter `yaml:"filter,omitempty"`
}

// StateDir resolves the directory holding projects.yaml, in order:
//
//	THING_DATA_DIR            -> used verbatim (projects.yaml sits directly in it)
//	$XDG_STATE_HOME/thingd    -> when XDG_STATE_HOME is an absolute path
//	~/.local/state/thingd     -> fallback
func StateDir() (string, error) {
	if v := os.Getenv("THING_DATA_DIR"); v != "" {
		return v, nil
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "thingd"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve state dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "thingd"), nil
}

// File returns the resolved path to projects.yaml.
func File() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Registry is projects.yaml as a whole: the registered projects plus the
// defaults that apply to every one of them.
type Registry struct {
	Defaults *Defaults `yaml:"defaults,omitempty"`
	Projects []Project `yaml:"projects"`
}

// Defaults holds the settings a project entry inherits when it does not set them
// itself.
type Defaults struct {
	Filter *Filter `yaml:"filter,omitempty"`
}

// filter returns d's filter block, tolerating nil defaults.
func (d *Defaults) filter() *Filter {
	if d == nil {
		return nil
	}
	return d.Filter
}

// Filters returns the filter state project p's board starts from, layering its
// entry over the registry-wide defaults. An unknown name resolves to the
// defaults alone, which is what a project registered at runtime gets.
func (r *Registry) Filters(name string) *Filter {
	for i := range r.Projects {
		if r.Projects[i].Name == name {
			return ResolveFilter(r.Defaults.filter(), r.Projects[i].Filter)
		}
	}
	return ResolveFilter(r.Defaults.filter(), nil)
}

// Load reads and validates projects.yaml at path. A missing file is not an
// error; it yields an empty registry. Names must be URL-safe slugs and unique.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, err
	}
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if err := validate(r.Projects); err != nil {
		return nil, err
	}
	return &r, nil
}

// Save writes projects to projects.yaml at path, creating the enclosing state
// directory if needed. It validates the same way Load does, so a bad in-memory
// list is rejected before it touches disk rather than being written back and
// then failing to load. The write is atomic: it lands a temp file and renames.
func Save(path string, r *Registry) error {
	if err := validate(r.Projects); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), FileName+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// validate enforces the registry invariants shared by Load and Save: every name
// is a URL-safe slug and names are unique.
func validate(projects []Project) error {
	seen := make(map[string]bool, len(projects))
	for _, p := range projects {
		if p.Name == "" || slug.Slugify(p.Name) != p.Name {
			return fmt.Errorf("invalid project name %q: must be a URL-safe slug", p.Name)
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate project name %q", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
