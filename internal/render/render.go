// Package render turns a loaded tree into human-readable plain text.
package render

import (
	"strings"

	"github.com/muniere/thing/internal/model"
)

// statusBox is a compact, fixed-width marker for a node's status.
func statusBox(s model.Status) string {
	switch s {
	case model.Done:
		return "[x]"
	case model.Doing:
		return "[~]"
	case model.Paused:
		return "[-]"
	default:
		return "[ ]"
	}
}

// label renders a single node as "[status] Title (slug)" with an optional
// priority marker. An untitled node falls back to its slug.
func label(n *model.Node) string {
	title := n.Title
	if title == "" {
		title = n.Slug
	}
	s := statusBox(n.Status) + " " + title + " (" + n.Slug + ")"
	if n.Priority != "" {
		s += " !" + string(n.Priority)
	}
	return s
}

// Tree renders the whole tree as an indented outline headed by title.
func Tree(nodes []*model.Node, title string) string {
	if title == "" {
		title = "thing"
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	writeChildren(&b, nodes, "")
	return b.String()
}

func writeChildren(b *strings.Builder, nodes []*model.Node, prefix string) {
	for i, n := range nodes {
		last := i == len(nodes)-1
		branch, childPrefix := "├─ ", prefix+"│  "
		if last {
			branch, childPrefix = "└─ ", prefix+"   "
		}
		b.WriteString(prefix + branch + label(n) + "\n")
		writeChildren(b, n.Children, childPrefix)
	}
}
