// Package exporter serializes the whole tree to JSON.
package exporter

import (
	"encoding/json"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// node is the JSON shape of one exported node. Optional fields are omitted when
// empty. Status is the node's own stored status — empty (and omitted) when the
// node has none, i.e. its displayed status is rolled up from its children; this
// is the canonical value that round-trips through export/import. EffectiveStatus
// is the derived display status (own-or-rollup) and Files lists the node's
// attachment file names; both are read-time derivations from disk, not stored
// data, so only the web serialization emits them (for the client's convenience)
// and the interchange Export leaves them out. Ref is the node's full slug-path
// identity, so a client (e.g. thingd's web UI) can address it; the bare slug is
// its last segment and is not emitted separately.
type node struct {
	Type            model.NodeType `json:"type"`
	Ref             string         `json:"ref"`
	Title           string         `json:"title"`
	Status          model.Status   `json:"status,omitempty"`
	EffectiveStatus model.Status   `json:"effectiveStatus,omitempty"`
	Priority        model.Priority `json:"priority,omitempty"`
	Category        string         `json:"category,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Updated         string         `json:"updated,omitempty"`
	Links           []model.Link   `json:"links,omitempty"`
	Body            string         `json:"body,omitempty"`
	Files           []string       `json:"files,omitempty"`
	Children        []node         `json:"children,omitempty"`
}

// Export loads the whole tree and returns it as an indented JSON array of
// top-level nodes (epics and orphan issues). It is the interchange format —
// consumed by `thing export` and mirrored by import — so it carries only stored
// data: each node's own status, never the derived rollup.
func Export(s *store.Store) ([]byte, error) {
	return export(s, false)
}

// ExportWeb is Export plus each node's effectiveStatus (the derived display
// status) and files (its attachment file names). thingd serves this to the web
// UI so the client does not recompute the rollup or list the directory itself;
// neither field is part of the interchange format (see Export).
func ExportWeb(s *store.Store) ([]byte, error) {
	return export(s, true)
}

func export(s *store.Store, effective bool) ([]byte, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := make([]node, 0, len(top))
	for _, n := range top {
		parentRef := ""
		if n.Type != model.Epic {
			parentRef = store.OrphanDir // a top-level non-epic is an orphan issue
		}
		out = append(out, convert(parentRef, n, effective))
	}
	return json.MarshalIndent(out, "", "  ")
}

func convert(parentRef string, n *model.Node, effective bool) node {
	ref := n.Slug
	if parentRef != "" {
		ref = parentRef + "/" + n.Slug
	}
	out := node{
		Type:     n.Type,
		Ref:      ref,
		Title:    n.Title,
		Status:   n.Status,
		Priority: n.Priority,
		Category: n.Category,
		Tags:     n.Tags,
		Updated:  n.Updated,
		Links:    n.Links,
		Body:     n.Body,
	}
	if effective {
		out.EffectiveStatus = n.EffectiveStatus()
		out.Files = n.Files
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, convert(ref, c, effective))
	}
	return out
}
