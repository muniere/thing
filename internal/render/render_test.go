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
