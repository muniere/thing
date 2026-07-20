// Package render turns a loaded tree into human-readable plain text.
package render

import (
	"fmt"
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
	s := statusBox(n.EffectiveStatus()) + " " + title + " (" + n.Slug + ")"
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

// List renders nodes as a flat, one-line-per-node plain-text listing.
func List(nodes []*model.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(listLine(n))
		b.WriteByte('\n')
	}
	return b.String()
}

func listLine(n *model.Node) string {
	s := statusBox(n.EffectiveStatus()) + " " + n.Slug
	if n.Title != "" && n.Title != n.Slug {
		s += "  " + n.Title
	}
	if n.Priority != "" {
		s += " !" + string(n.Priority)
	}
	return s
}

// Links renders a node's related links as a numbered list.
func Links(links []model.Link) string {
	var b strings.Builder
	for i, l := range links {
		if l.Label != "" {
			fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, l.URL, l.Label)
		} else {
			fmt.Fprintf(&b, "%d. %s\n", i+1, l.URL)
		}
	}
	return b.String()
}

// Show renders a single node's fields followed by its Markdown body.
func Show(n *model.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", n.Type, n.Slug)
	field(&b, "title", n.Title)
	field(&b, "status", string(n.EffectiveStatus()))
	field(&b, "priority", string(n.Priority))
	field(&b, "category", n.Category)
	if len(n.Tags) > 0 {
		field(&b, "tags", strings.Join(n.Tags, ", "))
	}
	field(&b, "updated", n.Updated)
	if len(n.Links) > 0 {
		b.WriteString("links:\n")
		for _, l := range n.Links {
			if l.Label != "" {
				fmt.Fprintf(&b, "  - %s (%s)\n", l.URL, l.Label)
			} else {
				fmt.Fprintf(&b, "  - %s\n", l.URL)
			}
		}
	}
	if body := strings.TrimRight(n.Body, "\n"); strings.TrimSpace(body) != "" {
		b.WriteByte('\n')
		b.WriteString(body)
		b.WriteByte('\n')
	}
	return b.String()
}

// field writes an aligned "name: value" line, skipping empty values.
func field(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%-9s %s\n", name+":", value)
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
