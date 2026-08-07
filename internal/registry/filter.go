package registry

import (
	"fmt"
	"slices"
	"strings"

	"github.com/muniere/thing/internal/model"
	"gopkg.in/yaml.v3"
)

// filterKeys lists every key the filter block accepts.
var filterKeys = []string{"statuses", "priorities", "category", "tag", "query"}

// Filter is the filter state a board starts from, configured under a `filter`
// key — once at the top level of projects.yaml as the default for every project,
// and again on a project's own entry to override it.
//
// Whether a key was written matters independently of its value: when an entry is
// layered over the defaults (see ResolveFilter), an omitted key inherits while a
// written one wins even if it is empty. Values are plain, so "" never has to
// stand for "absent"; set records presence instead.
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

// MarshalYAML emits only the keys that were written, so a filter thingd read and
// wrote back is the same block it started from — an omitted key does not
// reappear as an empty value, which would turn "inherit" into "clear".
func (f *Filter) MarshalYAML() (any, error) {
	if f == nil {
		return nil, nil
	}
	out := make(map[string]any, len(f.set))
	for _, key := range filterKeys {
		if !f.Has(key) {
			continue
		}
		switch key {
		case "statuses":
			out[key] = f.Statuses
		case "priorities":
			out[key] = f.Priorities
		case "category":
			out[key] = f.Category
		case "tag":
			out[key] = f.Tag
		case "query":
			out[key] = f.Query
		}
	}
	return out, nil
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

// ResolveFilter layers a project entry's filter over the registry-wide defaults,
// key by key: a key the entry wrote wins — even when empty, which clears the
// inherited value — and a key it omitted falls back to the defaults. It returns
// nil when neither carries a filter block, so the caller can leave it out
// entirely.
func ResolveFilter(defaults, project *Filter) *Filter {
	if defaults == nil && project == nil {
		return nil
	}
	out := &Filter{set: make(map[string]bool, len(filterKeys))}
	for _, key := range filterKeys {
		src := defaults
		if project.Has(key) {
			src = project
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
