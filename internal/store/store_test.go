package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/muniere/thing/internal/model"
)

// write creates a node file at the given path, making parent dirs as needed.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Epic "alpha" with two issues.
	write(t, filepath.Join(root, "alpha", "_epic.md"), "---\ntitle: Alpha\npriority: high\n---\nEpic body\n")
	write(t, filepath.Join(root, "alpha", "one", "_issue.md"), "---\ntitle: One\nstatus: done\n---\n")
	write(t, filepath.Join(root, "alpha", "one", "task-a.md"), "---\ntitle: Task A\nstatus: done\n---\n")
	write(t, filepath.Join(root, "alpha", "two", "_issue.md"), "---\ntitle: Two\nstatus: doing\n---\n")
	// Orphan issue.
	write(t, filepath.Join(root, OrphanDir, "loose", "_issue.md"), "---\ntitle: Loose\n---\n")
	write(t, filepath.Join(root, OrphanDir, "loose", "z-task.md"), "---\ntitle: Z\n---\n")
	write(t, filepath.Join(root, OrphanDir, "loose", "a-task.md"), "---\ntitle: A\n---\n")
	// Non-node clutter that must be ignored.
	write(t, filepath.Join(root, "config.yaml"), "title: Test\n")
	return root
}

func TestLoad(t *testing.T) {
	root := fixture(t)
	top, err := Open(root).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Top level sorted by slug: "alpha" (epic) then "loose" (orphan issue).
	if len(top) != 2 {
		t.Fatalf("top-level count = %d, want 2", len(top))
	}
	epic := top[0]
	if epic.Type != model.Epic || epic.Slug != "alpha" || epic.Title != "Alpha" {
		t.Errorf("epic = %+v", epic)
	}
	if len(epic.Children) != 2 {
		t.Fatalf("epic children = %d, want 2", len(epic.Children))
	}
	if epic.Children[0].Slug != "one" || epic.Children[1].Slug != "two" {
		t.Errorf("issue order = %s, %s", epic.Children[0].Slug, epic.Children[1].Slug)
	}
	if got := epic.Children[0].Children; len(got) != 1 || got[0].Slug != "task-a" || got[0].Type != model.Task {
		t.Errorf("task load = %+v", got)
	}

	orphan := top[1]
	if orphan.Type != model.Issue || orphan.Slug != "loose" {
		t.Errorf("orphan = %+v", orphan)
	}
	// Tasks sorted by slug.
	if len(orphan.Children) != 2 || orphan.Children[0].Slug != "a-task" || orphan.Children[1].Slug != "z-task" {
		t.Errorf("orphan tasks = %+v", orphan.Children)
	}
	// Orphan issue with no explicit status defaults to todo.
	if orphan.Status != model.Todo {
		t.Errorf("orphan status = %q, want todo", orphan.Status)
	}
}

func TestRollup(t *testing.T) {
	root := fixture(t)
	top, _ := Open(root).Load()
	// alpha has issue "one"=done and "two"=doing -> any doing -> doing.
	if top[0].Status != model.Doing {
		t.Errorf("alpha rollup = %q, want doing", top[0].Status)
	}
}

func TestRollupExplicitWins(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "e", "_epic.md"), "---\ntitle: E\nstatus: paused\n---\n")
	write(t, filepath.Join(root, "e", "i", "_issue.md"), "---\ntitle: I\nstatus: doing\n---\n")
	top, _ := Open(root).Load()
	if top[0].Status != model.Paused {
		t.Errorf("explicit epic status = %q, want paused", top[0].Status)
	}
}

func TestRollupStatusRules(t *testing.T) {
	mk := func(sts ...model.Status) []*model.Node {
		var ns []*model.Node
		for _, s := range sts {
			ns = append(ns, &model.Node{Status: s})
		}
		return ns
	}
	cases := []struct {
		name string
		in   []*model.Node
		want model.Status
	}{
		{"empty", nil, model.Todo},
		{"all done", mk(model.Done, model.Done), model.Done},
		{"all todo", mk(model.Todo, model.Todo), model.Todo},
		{"any doing", mk(model.Todo, model.Doing, model.Done), model.Doing},
		{"mixed done/todo", mk(model.Done, model.Todo), model.Doing},
	}
	for _, c := range cases {
		if got := rollupStatus(c.in); got != c.want {
			t.Errorf("%s: rollupStatus = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	if got, _ := Resolve("/explicit", false); got != "/explicit" {
		t.Errorf("dir flag: got %q", got)
	}

	t.Setenv("THING_DIR", "/from/env")
	if got, _ := Resolve("", false); got != "/from/env" {
		t.Errorf("THING_DIR: got %q", got)
	}
	// dir flag beats env.
	if got, _ := Resolve("/explicit", false); got != "/explicit" {
		t.Errorf("dir flag over env: got %q", got)
	}
}

func TestResolveUpwardSearch(t *testing.T) {
	t.Setenv("THING_DIR", "")
	root := t.TempDir()
	// Resolve symlinks so macOS /var -> /private/var doesn't defeat the compare.
	root, _ = filepath.EvalSymlinks(root)
	thing := filepath.Join(root, ".thing")
	if err := os.MkdirAll(thing, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != thing {
		t.Errorf("upward search: got %q, want %q", got, thing)
	}
}
