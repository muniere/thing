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

// Project is one registered project: a URL-safe name and its data directory.
type Project struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
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

type projectsFile struct {
	Projects []Project `yaml:"projects"`
}

// Load reads and validates projects.yaml at path. A missing file is not an error;
// it yields an empty slice. Names must be URL-safe slugs and unique.
func Load(path string) ([]Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pf projectsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(pf.Projects))
	for _, p := range pf.Projects {
		if p.Name == "" || slug.Slugify(p.Name) != p.Name {
			return nil, fmt.Errorf("invalid project name %q: must be a URL-safe slug", p.Name)
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("duplicate project name %q", p.Name)
		}
		seen[p.Name] = true
	}
	return pf.Projects, nil
}
