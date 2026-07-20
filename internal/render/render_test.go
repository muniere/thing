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
	out := Tree(nodes, "My Board")

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
	out := Tree(nil, "")
	if !strings.HasPrefix(out, "thing\n") {
		t.Errorf("default title: %q", out)
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
