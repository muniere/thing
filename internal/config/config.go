// Package config loads the per-directory config.yaml (title and the exclusive
// category groups used to organize epics).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/muniere/thing/internal/model"
	"gopkg.in/yaml.v3"
)

// FileName is the config file's name within the data directory.
const FileName = "config.yaml"

// Config holds the settings stored in config.yaml.
type Config struct {
	Title      string   `yaml:"title,omitempty"`
	Categories []string `yaml:"categories,omitempty"`
	Filter     *Filter  `yaml:"filter,omitempty"`
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

// filterKeys lists every key the filter block accepts.
var filterKeys = []string{"statuses", "priorities", "category", "tag", "query"}

// Filter is the filter state the web UI starts from, configured under the
// `filter` key. Whether a key was written matters independently of its value:
// when a project config is layered over the global one (see ResolveFilter), an
// omitted key inherits while a written one wins even if it is empty. Values are
// plain, so "" never has to stand for "absent"; set records presence instead.
type Filter struct {
	Statuses   []model.Status
	Priorities []model.Priority
	Category   string
	Tag        string
	Query      string

	set map[string]bool
}

// Has reports whether key (one of filterKeys) was written in the YAML. A nil
// Filter — an omitted block — has nothing.
func (f *Filter) Has(key string) bool {
	return f != nil && f.set[key]
}

// UnmarshalYAML decodes the filter block through a map of raw nodes so presence
// is read from the mapping itself rather than inferred from a zero value. Unknown
// keys and invalid status/priority names are errors: a typo would otherwise show
// up as a silently empty board.
func (f *Filter) UnmarshalYAML(n *yaml.Node) error {
	var raw map[string]yaml.Node
	if err := n.Decode(&raw); err != nil {
		return fmt.Errorf("filter: %w", err)
	}
	f.set = make(map[string]bool, len(raw))
	for k := range raw {
		if !slices.Contains(filterKeys, k) {
			return fmt.Errorf("filter: unknown key %q (want %s)", k, strings.Join(filterKeys, "|"))
		}
		f.set[k] = true
	}
	if err := decodeFilterKey(raw, "statuses", &f.Statuses); err != nil {
		return err
	}
	if err := decodeFilterKey(raw, "priorities", &f.Priorities); err != nil {
		return err
	}
	if err := decodeFilterKey(raw, "category", &f.Category); err != nil {
		return err
	}
	if err := decodeFilterKey(raw, "tag", &f.Tag); err != nil {
		return err
	}
	if err := decodeFilterKey(raw, "query", &f.Query); err != nil {
		return err
	}
	return f.validate()
}

// decodeFilterKey decodes raw[key] into dst. A missing or explicitly null node
// leaves dst at its zero value; the key's presence is recorded separately, so an
// empty value still means "do not filter this facet".
func decodeFilterKey(raw map[string]yaml.Node, key string, dst any) error {
	n, ok := raw[key]
	if !ok || n.Tag == "!!null" {
		return nil
	}
	if err := n.Decode(dst); err != nil {
		return fmt.Errorf("filter.%s: %w", key, err)
	}
	return nil
}

// validate rejects unknown status and priority names.
func (f *Filter) validate() error {
	for _, s := range f.Statuses {
		if !s.Valid() {
			return fmt.Errorf("filter.statuses: invalid status %q (want %s)", s, model.StatusValues())
		}
	}
	for _, p := range f.Priorities {
		if !p.Valid() {
			return fmt.Errorf("filter.priorities: invalid priority %q (want %s)", p, model.PriorityValues())
		}
	}
	return nil
}

// filter returns c's filter block, tolerating a nil config.
func (c *Config) filter() *Filter {
	if c == nil {
		return nil
	}
	return c.Filter
}

// ResolveFilter layers a project's filter defaults over the global ones, key by
// key: a key the project config wrote wins — even when empty, which clears the
// inherited value — and a key it omitted falls back to global. It returns nil when
// neither config carries a filter block, so the caller can leave it out entirely.
//
// Only `filter` is layered this way. Title and categories keep resolving to a
// single config.yaml (the CLI's first-match rule), so the CLI and the web never
// disagree about them.
func ResolveFilter(global, project *Config) *Filter {
	g, p := global.filter(), project.filter()
	if g == nil && p == nil {
		return nil
	}
	out := &Filter{set: make(map[string]bool, len(filterKeys))}
	for _, key := range filterKeys {
		src := g
		if p.Has(key) {
			src = p
		}
		if !src.Has(key) {
			continue
		}
		out.set[key] = true
		switch key {
		case "statuses":
			out.Statuses = src.Statuses
		case "priorities":
			out.Priorities = src.Priorities
		case "category":
			out.Category = src.Category
		case "tag":
			out.Tag = src.Tag
		case "query":
			out.Query = src.Query
		}
	}
	return out
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
