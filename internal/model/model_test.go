package model

import "testing"

func TestNodeTypeValid(t *testing.T) {
	for _, ty := range []NodeType{Epic, Issue, Task} {
		if !ty.Valid() {
			t.Errorf("NodeType %q should be valid", ty)
		}
	}
	if NodeType("nope").Valid() {
		t.Error("unknown NodeType should be invalid")
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range Statuses {
		if !s.Valid() {
			t.Errorf("Status %q should be valid", s)
		}
	}
	if Status("").Valid() || Status("blocked").Valid() {
		t.Error("unknown Status should be invalid")
	}
	if len(Statuses) != 4 {
		t.Errorf("expected 4 statuses, got %d", len(Statuses))
	}
}

func TestPriorityValid(t *testing.T) {
	for _, p := range Priorities {
		if !p.Valid() {
			t.Errorf("Priority %q should be valid", p)
		}
	}
	if Priority("urgent").Valid() {
		t.Error("unknown Priority should be invalid")
	}
}

func TestEffectiveStatus(t *testing.T) {
	// A non-epic falls back to todo when its status is empty, or reports its own.
	if got := (&Node{Type: Task}).EffectiveStatus(); got != Todo {
		t.Errorf("empty task status = %q, want todo", got)
	}
	if got := (&Node{Type: Task, Status: Done}).EffectiveStatus(); got != Done {
		t.Errorf("explicit task status = %q, want done", got)
	}

	// An epic without an explicit status rolls its issues up:
	// all done -> done; any doing -> doing; all todo -> todo; otherwise doing.
	issues := func(sts ...Status) []*Node {
		var ns []*Node
		for _, s := range sts {
			ns = append(ns, &Node{Type: Issue, Status: s})
		}
		return ns
	}
	cases := []struct {
		name string
		in   []*Node
		want Status
	}{
		{"empty", nil, Todo},
		{"all done", issues(Done, Done), Done},
		{"all todo", issues(Todo, Todo), Todo},
		{"any doing", issues(Todo, Doing, Done), Doing},
		{"mixed done/todo", issues(Done, Todo), Doing},
		{"blank issue treated as todo", issues("", Done), Doing},
	}
	for _, c := range cases {
		epic := &Node{Type: Epic, Children: c.in}
		if got := epic.EffectiveStatus(); got != c.want {
			t.Errorf("%s: rollup = %q, want %q", c.name, got, c.want)
		}
	}

	// An explicit epic status wins over the rollup.
	pinned := &Node{Type: Epic, Status: Paused, Children: issues(Doing)}
	if got := pinned.EffectiveStatus(); got != Paused {
		t.Errorf("pinned epic status = %q, want paused", got)
	}
}
