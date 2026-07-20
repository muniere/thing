// Package model defines the core domain types for a thing tree:
// the Epic > Issue > Task node and the small enumerations that describe it.
package model

import "strings"

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
}

// EffectiveStatus is the status to display for a node. An explicit Status wins.
// Otherwise an epic rolls its status up from its issues, and any other node
// defaults to Todo. It is a read-time derivation and is never persisted, so
// setting an epic's priority does not freeze its rolled-up status onto disk.
func (n *Node) EffectiveStatus() Status {
	if n.Status != "" {
		return n.Status
	}
	if n.Type == Epic {
		return rollup(n.Children)
	}
	return Todo
}

// rollup derives an epic's status from its issues:
// all done -> done; any doing -> doing; all todo -> todo; otherwise doing.
func rollup(issues []*Node) Status {
	if len(issues) == 0 {
		return Todo
	}
	allDone, allTodo, anyDoing := true, true, false
	for _, is := range issues {
		st := is.EffectiveStatus()
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
