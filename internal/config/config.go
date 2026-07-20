// Package config loads the per-directory config.yaml (title and the exclusive
// category groups used to organize epics).
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
