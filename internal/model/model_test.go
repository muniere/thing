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
