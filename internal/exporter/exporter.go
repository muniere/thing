// Package exporter serializes the whole tree to JSON.
package exporter

import (
	"encoding/json"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// node is the JSON shape of one exported node. Optional fields are omitted when
// empty; status is always the effective (displayed) status.
type node struct {
	Type     model.NodeType `json:"type"`
	Slug     string         `json:"slug"`
	Title    string         `json:"title"`
	Status   model.Status   `json:"status"`
	Priority model.Priority `json:"priority,omitempty"`
	Category string         `json:"category,omitempty"`
	Tags     []string       `json:"tags,omitempty"`
	Updated  string         `json:"updated,omitempty"`
	Links    []model.Link   `json:"links,omitempty"`
	Body     string         `json:"body,omitempty"`
	Children []node         `json:"children,omitempty"`
}

// Export loads the whole tree and returns it as an indented JSON array of
// top-level nodes (epics and orphan issues).
func Export(s *store.Store) ([]byte, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := make([]node, 0, len(top))
	for _, n := range top {
		out = append(out, convert(n))
	}
	return json.MarshalIndent(out, "", "  ")
}

func convert(n *model.Node) node {
	out := node{
		Type:     n.Type,
		Slug:     n.Slug,
		Title:    n.Title,
		Status:   n.EffectiveStatus(),
		Priority: n.Priority,
		Category: n.Category,
		Tags:     n.Tags,
		Updated:  n.Updated,
		Links:    n.Links,
		Body:     n.Body,
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, convert(c))
	}
	return out
}
