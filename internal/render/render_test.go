package render

import (
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
)

func TestTree(t *testing.T) {
	nodes := []*model.Node{
		{
			Type: model.Epic, Slug: "alpha", Title: "Alpha", Status: model.Doing, Priority: model.High,
			Children: []*model.Node{
				{Type: model.Issue, Slug: "one", Title: "One", Status: model.Done, Children: []*model.Node{
					{Type: model.Task, Slug: "task-a", Title: "Task A", Status: model.Done},
				}},
				{Type: model.Issue, Slug: "two", Title: "Two", Status: model.Doing},
			},
		},
		{Type: model.Issue, Slug: "loose", Status: model.Todo},
	}
	out := Tree(nodes, "My Board", nil)

	wantLines := []string{
		"My Board",
		"├─ [~] Alpha (alpha) !high",
		"│  ├─ [x] One (one)",
		"│  │  └─ [x] Task A (task-a)",
		"│  └─ [~] Two (two)",
		"└─ [ ] loose (loose)", // untitled falls back to slug
	}
	got := out
	for _, w := range wantLines {
		if !strings.Contains(got, w) {
			t.Errorf("tree missing line %q\nfull output:\n%s", w, got)
		}
	}
	// Exact structure check.
	want := strings.Join(wantLines, "\n") + "\n"
	if got != want {
		t.Errorf("tree output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestTreeDefaultTitle(t *testing.T) {
	out := Tree(nil, "", nil)
	if !strings.HasPrefix(out, "thing\n") {
		t.Errorf("default title: %q", out)
	}
}

func TestTreeCategoriesHangOffTitle(t *testing.T) {
	top := []*model.Node{
		{Type: model.Epic, Slug: "web", Title: "Web", Category: "Project"},
		{Type: model.Epic, Slug: "misc", Title: "Misc"}, // no category -> uncategorized
	}
	out := Tree(top, "Board", []string{"Project"})
	// The title is the root; each category heading is a branch under it with its
	// epics hanging beneath.
	want := strings.Join([]string{
		"Board",
		"├─ # Project",
		"│  └─ [ ] Web (web)",
		"└─ # (uncategorized)",
		"   └─ [ ] Misc (misc)",
	}, "\n") + "\n"
	if out != want {
		t.Errorf("category tree mismatch:\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestList(t *testing.T) {
	nodes := []*model.Node{
		{Slug: "one", Title: "One", Status: model.Done, Priority: model.High},
		{Slug: "two", Status: model.Todo}, // untitled: no title suffix
	}
	got := List(nodes)
	want := "[x] one  One !high\n[ ] two\n"
	if got != want {
		t.Errorf("List = %q, want %q", got, want)
	}
}

func TestShow(t *testing.T) {
	n := &model.Node{
		Type: model.Issue, Slug: "monitor", Title: "Monitor", Status: model.Doing,
		Priority: model.High, Tags: []string{"a", "b"}, Updated: "2026-07-19",
		Links: []model.Link{{URL: "https://x.test", Label: "Doc"}, {URL: "https://y.test"}},
		Body:  "Body line.\n",
	}
	got := Show(n)
	for _, want := range []string{
		"issue monitor\n",
		"title:    Monitor\n",
		"status:   doing\n",
		"tags:     a, b\n",
		"links:\n",
		"  - https://x.test (Doc)\n",
		"  - https://y.test\n",
		"\nBody line.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Show missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestShowNoBody(t *testing.T) {
	got := Show(&model.Node{Type: model.Task, Slug: "t", Title: "T", Status: model.Todo})
	if strings.Contains(got, "priority:") {
		t.Errorf("empty priority should be omitted: %q", got)
	}
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("no trailing blank line without body: %q", got)
	}
}

func TestCategoryGrouping(t *testing.T) {
	top := []*model.Node{
		{Type: model.Epic, Slug: "web", Title: "Web", Category: "Project"},
		{Type: model.Epic, Slug: "home", Title: "Home", Category: "Personal"},
		{Type: model.Epic, Slug: "misc", Title: "Misc"},                 // no category
		{Type: model.Epic, Slug: "old", Title: "Old", Category: "Gone"}, // unknown category
		{Type: model.Issue, Slug: "loose", Title: "Loose"},              // orphan issue
	}
	// "Archive" is configured but unused: its heading must be skipped.
	cats := []string{"Project", "Personal", "Archive"}

	// Tree groups under category headings in config order, with the empty/unknown
	// epics and orphans under "(uncategorized)" last.
	tree := Tree(top, "Board", cats)
	if strings.Contains(tree, "# Archive") {
		t.Errorf("a configured category with no epics must not emit a heading:\n%s", tree)
	}
	for _, want := range []string{"# Project", "# Personal", "# (uncategorized)"} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree missing heading %q:\n%s", want, tree)
		}
	}
	if i, j, k := strings.Index(tree, "# Project"), strings.Index(tree, "# Personal"), strings.Index(tree, "# (uncategorized)"); !(i < j && j < k) {
		t.Errorf("heading order wrong: Project=%d Personal=%d uncat=%d", i, j, k)
	}
	// misc, old, and the orphan all land in uncategorized.
	uncat := tree[strings.Index(tree, "# (uncategorized)"):]
	for _, want := range []string{"misc", "old", "loose"} {
		if !strings.Contains(uncat, want) {
			t.Errorf("uncategorized missing %q:\n%s", want, uncat)
		}
	}

	// TopList groups the same way, flatly. Assert the exact layout: no leading
	// blank line, a blank line between groups, and the empty "Archive" skipped.
	want := "# Project\n[ ] web  Web\n\n" +
		"# Personal\n[ ] home  Home\n\n" +
		"# (uncategorized)\n[ ] misc  Misc\n[ ] old  Old\n[ ] loose  Loose\n"
	if got := TopList(top, cats); got != want {
		t.Errorf("TopList layout:\n got: %q\nwant: %q", got, want)
	}

	// Categories configured but no epic matches any: only "(uncategorized)".
	allUncat := TopList([]*model.Node{{Type: model.Epic, Slug: "x", Title: "X"}}, cats)
	if strings.Contains(allUncat, "# Project") || !strings.HasPrefix(allUncat, "# (uncategorized)\n") {
		t.Errorf("all-uncategorized: %q", allUncat)
	}

	// With no categories configured, output stays flat (no headings).
	if flat := Tree(top, "Board", nil); strings.Contains(flat, "#") {
		t.Errorf("no categories should render flat:\n%s", flat)
	}
	if flat := TopList(top, nil); strings.Contains(flat, "#") {
		t.Errorf("no categories should render a flat list:\n%s", flat)
	}
}
