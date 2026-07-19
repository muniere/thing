// Package model defines the core domain types for a thing tree:
// the Epic > Issue > Task node and the small enumerations that describe it.
package model

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
// node file's YAML frontmatter.
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
