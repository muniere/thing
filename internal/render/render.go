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

// uncategorizedHeading groups epics with no (or unknown) category, plus orphan
// issues, when category grouping is active.
const uncategorizedHeading = "(uncategorized)"

// group is a set of top-level nodes shown under one heading ("" = no heading).
type group struct {
	heading string
	nodes   []*model.Node
}

// groupTop partitions top-level nodes into category groups. With no categories
// configured it returns a single unheaded group (flat rendering). Otherwise
// epics are grouped under their category in config order; epics with an empty
// or unknown category and all orphan issues fall under "(uncategorized)".
func groupTop(top []*model.Node, categories []string) []group {
	if len(categories) == 0 {
		return []group{{nodes: top}}
	}
	var epics, orphans []*model.Node
	for _, n := range top {
		if n.Type == model.Epic {
			epics = append(epics, n)
		} else {
			orphans = append(orphans, n)
		}
	}
	known := make(map[string]bool, len(categories))
	for _, c := range categories {
		known[c] = true
	}

	var groups []group
	for _, c := range categories {
		var g []*model.Node
		for _, e := range epics {
			if e.Category == c {
				g = append(g, e)
			}
		}
		if len(g) > 0 {
			groups = append(groups, group{heading: c, nodes: g})
		}
	}
	var rest []*model.Node
	for _, e := range epics {
		if e.Category == "" || !known[e.Category] {
			rest = append(rest, e)
		}
	}
	rest = append(rest, orphans...)
	if len(rest) > 0 {
		groups = append(groups, group{heading: uncategorizedHeading, nodes: rest})
	}
	return groups
}

// Tree renders the whole tree as an indented outline rooted at title: the
// top-level nodes hang off the title. When categories are configured, each
// category heading is an intermediate branch under the title with its epics
// beneath it; otherwise the top-level nodes hang off the title directly.
func Tree(nodes []*model.Node, title string, categories []string) string {
	if title == "" {
		title = "thing"
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	groups := groupTop(nodes, categories)
	// No headings (no categories configured): the nodes hang off the title.
	if len(groups) == 1 && groups[0].heading == "" {
		writeChildren(&b, groups[0].nodes, "")
		return b.String()
	}
	// Otherwise each category heading is a branch under the title, its epics
	// hanging under it.
	for i, g := range groups {
		branch, childPrefix := "├─ ", "│  "
		if i == len(groups)-1 {
			branch, childPrefix = "└─ ", "   "
		}
		b.WriteString(branch + "# " + g.heading + "\n")
		writeChildren(&b, g.nodes, childPrefix)
	}
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

// TopList renders top-level nodes as a flat listing, grouped under category
// headings when categories are configured.
func TopList(nodes []*model.Node, categories []string) string {
	var b strings.Builder
	for _, g := range groupTop(nodes, categories) {
		if g.heading != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("# " + g.heading + "\n")
		}
		for _, n := range g.nodes {
			b.WriteString(listLine(n))
			b.WriteByte('\n')
		}
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
