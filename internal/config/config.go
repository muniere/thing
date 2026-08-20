// Package config loads the per-directory config.yaml: the title and the
// exclusive category groups used to organize epics.
//
// These describe what a tree is, so both the CLI and thingd read them. Settings
// that only shape a thingd board — its starting filter state — are not here;
// they belong to the server and live on the project's entry in projects.yaml.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName is the config file's name within the data directory.
const FileName = "config.yaml"

// Config holds the settings stored in config.yaml.
type Config struct {
	Title      string   `yaml:"title,omitempty"`
	Categories []string `yaml:"categories,omitempty"`
}

// iconNames are the file names a tree may use for its own board icon, in the
// order they are preferred. The icon is found by convention rather than named in
// config.yaml so that dropping the file in is the whole of the configuration;
// SVG comes first because it stays sharp at every size a tab or a dock asks for,
// while a PNG is fixed at whatever it was exported at.
var iconNames = []string{"icon.svg", "icon.png"}

// Icon reports the name of the icon file root carries, or ok false when it
// carries none — the common case, which leaves the board on its built-in mark.
// It returns the name rather than the contents because the caller serving it
// wants the path anyway, to hand the file to the http layer as a file.
func Icon(root string) (name string, ok bool) {
	for _, name := range iconNames {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && info.Mode().IsRegular() {
			return name, true
		}
	}
	return "", false
}

// Load reads config.yaml from root. A missing file is not an error; it yields a
// zero-value Config.
func Load(root string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

const starter = `# thing configuration
title: thing

# Exclusive category groups for epics, used to organize tree/list output.
# Each epic may belong to at most one category.
categories: []
`

// Starter returns the contents written by ` + "`thing init`" + `.
func Starter() []byte {
	return []byte(starter)
}

// serverKeys are keys that used to live in config.yaml but now belong to thingd's
// registry (projects.yaml), where the settings that shape a board — as opposed to
// describing the tree — are configured.
var serverKeys = []string{"filter"}

// StaleServerKeys reports which server-only keys a config.yaml still carries.
// They are ignored on load, so without this a filter left behind after the move
// would simply stop working with nothing said; thingd warns about what it finds.
// A missing or unreadable file has nothing to report.
func StaleServerKeys(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		return nil
	}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var found []string
	for _, key := range serverKeys {
		if _, ok := raw[key]; ok {
			found = append(found, key)
		}
	}
	return found
}
