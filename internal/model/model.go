// Package model defines the core domain types for a thing tree:
// the Epic > Issue > Task node and the small enumerations that describe it.
package model

import (
	"strings"

	"github.com/muniere/thing/internal/section"
)

// NodeType is the kind of a node in the tree.
type NodeType string

const (
	Epic  NodeType = "epic"
	Issue NodeType = "issue"
	Task  NodeType = "task"
)

// Valid reports whether t is one of the known node types.
func (t NodeType) Valid() bool {
	switch t {
	case Epic, Issue, Task:
		return true
	}
	return false
}

// Status is the workflow state of a node.
type Status string

const (
	Todo   Status = "todo"
	Doing  Status = "doing"
	Done   Status = "done"
	Paused Status = "paused"
)

// Statuses lists every valid status in display order.
var Statuses = []Status{Todo, Doing, Done, Paused}

// StatusValues renders the valid statuses as a "todo|doing|done|paused" hint
// for error messages, kept in sync with Statuses.
func StatusValues() string {
	parts := make([]string, len(Statuses))
	for i, s := range Statuses {
		parts[i] = string(s)
	}
	return strings.Join(parts, "|")
}

// Valid reports whether s is one of the known statuses.
func (s Status) Valid() bool {
	switch s {
	case Todo, Doing, Done, Paused:
		return true
	}
	return false
}

// Priority is the relative importance of a node.
type Priority string

const (
	High   Priority = "high"
	Medium Priority = "medium"
	Low    Priority = "low"
)

// Priorities lists every valid priority in display order.
var Priorities = []Priority{High, Medium, Low}

// PriorityValues renders the valid priorities as a "high|medium|low" hint for
// error messages, kept in sync with Priorities.
func PriorityValues() string {
	parts := make([]string, len(Priorities))
	for i, p := range Priorities {
		parts[i] = string(p)
	}
	return strings.Join(parts, "|")
}

// Valid reports whether p is one of the known priorities.
func (p Priority) Valid() bool {
	switch p {
	case High, Medium, Low:
		return true
	}
	return false
}

// Link is a related URL attached to a node, independent of its body.
type Link struct {
	URL   string `yaml:"url" json:"url"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// Node is a single node in the tree. Type, Slug, Body, and Children are
// derived from the on-disk layout; the remaining fields are stored in the
// node file's YAML frontmatter. Status is the file's explicit status, empty
// when the file omits one; use EffectiveStatus for the value to display.
type Node struct {
	Type     NodeType
	Slug     string
	Title    string
	Status   Status
	Priority Priority
	Category string
	Tags     []string
	Updated  string
	Links    []Link
	Body     string
	Children []*Node
	Files    []string // attachment file names in the node's own directory (never for a task, which owns no directory)

	ArchivedRef string // the live-tree ref this node was archived from; empty on live nodes
	ArchivedAt  string // RFC3339 instant it was archived; empty on live nodes
}

// EffectiveStatus is the status to display for a node. An explicit Status wins.
// Otherwise a parent (epic or issue) rolls its status up from its children —
// an epic from its issues, an issue from its tasks — and a task defaults to
// Todo. Rollup recurses, so a statusless epic reflects its tasks through its
// issues. It is a read-time derivation and is never persisted, so setting a
// node's priority does not freeze its rolled-up status onto disk.
func (n *Node) EffectiveStatus() Status {
	if n.Status != "" {
		return n.Status
	}
	if n.Type == Epic || n.Type == Issue {
		return rollup(n.Children)
	}
	return Todo
}

// Markers is the body's section-convention warnings (a missing required
// heading, or headings out of the prescribed order), like EffectiveStatus a
// read-time derivation and never persisted: the convention is enforced by
// the "thing" skill, not by this tool, so a body is never rejected for
// failing it. Nil when the body raises nothing worth warning about.
func (n *Node) Markers() []section.Marker {
	return section.Check(n.Body)
}

// rollup derives a parent's status from its children's effective status:
// all done -> done; any doing -> doing; all todo -> todo; otherwise doing. The
// final case covers any mix as well as a paused child, so a paused child rolls
// up as doing.
func rollup(nodes []*Node) Status {
	if len(nodes) == 0 {
		return Todo
	}
	allDone, allTodo, anyDoing := true, true, false
	for _, n := range nodes {
		st := n.EffectiveStatus()
		if st != Done {
			allDone = false
		}
		if st != Todo {
			allTodo = false
		}
		if st == Doing {
			anyDoing = true
		}
	}
	switch {
	case allDone:
		return Done
	case anyDoing:
		return Doing
	case allTodo:
		return Todo
	default:
		return Doing
	}
}
