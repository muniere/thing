package model

import (
	"testing"
)

// wellFormedBody carries all four convention sections, in order.
const wellFormedBody = `## Summary

A short summary.

## Details

More detail.

## Definition of Done

- [ ] pending item
- [x] done item

## Comments

### //2026-01-02

second comment

### //2026-01-01

first comment
`

// legacyBody mirrors a real pre-convention node body (see
// .thing/thingd-project-switcher/_epic.md): a level-1 heading followed by
// non-canonical "## " sections with Japanese prose. None of its headings
// match the convention, so every required section is missing.
const legacyBody = `# thingd project switcher

開いているプロジェクト内から、左上ロゴ隣のキャレットで別プロジェクトへ切り替えられるようにする。

## 決定事項

- ロゴ ` + "`<a>`" + ` はプロジェクトルートへのリンクのまま維持
- [ ] 未整理のTODO

## 実装

` + "`Root.tsx`" + `: App へ ` + "`onSwitch`" + ` を渡す。

## 検証

` + "`npm run typecheck`" + ` で確認。
`

func TestMarkers(t *testing.T) {
	n := &Node{Body: wellFormedBody}
	if got := n.Markers(); got != nil {
		t.Errorf("Markers() = %+v, want nil", got)
	}

	legacy := &Node{Body: legacyBody}
	got := legacy.Markers()
	if len(got) != 3 {
		t.Fatalf("legacy Markers() = %+v, want 3 missing-section warnings", got)
	}
}

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

	// An issue rolls up from its tasks, just like an epic from its issues.
	tasks := func(sts ...Status) []*Node {
		var ns []*Node
		for _, s := range sts {
			ns = append(ns, &Node{Type: Task, Status: s})
		}
		return ns
	}
	if got := (&Node{Type: Issue, Children: tasks(Done, Done)}).EffectiveStatus(); got != Done {
		t.Errorf("issue rollup all-done = %q, want done", got)
	}
	if got := (&Node{Type: Issue, Children: tasks(Done, Todo)}).EffectiveStatus(); got != Doing {
		t.Errorf("issue rollup mixed = %q, want doing", got)
	}
	// An explicit issue status wins over its task rollup.
	if got := (&Node{Type: Issue, Status: Paused, Children: tasks(Done, Done)}).EffectiveStatus(); got != Paused {
		t.Errorf("pinned issue status = %q, want paused", got)
	}

	// Rollup recurses: a statusless epic reflects its tasks through its issues.
	deep := &Node{Type: Epic, Children: []*Node{
		{Type: Issue, Children: tasks(Done, Done)},
	}}
	if got := deep.EffectiveStatus(); got != Done {
		t.Errorf("recursive rollup = %q, want done", got)
	}
	// A mix two levels down propagates up as doing.
	deepMixed := &Node{Type: Epic, Children: []*Node{
		{Type: Issue, Children: tasks(Done, Todo)},
	}}
	if got := deepMixed.EffectiveStatus(); got != Doing {
		t.Errorf("recursive mixed rollup = %q, want doing", got)
	}
	// Recursion honors an intermediate issue's explicit status (its own status,
	// not a re-derived rollup of its tasks): a done issue with all-todo tasks
	// still makes the statusless epic done.
	deepPinned := &Node{Type: Epic, Children: []*Node{
		{Type: Issue, Status: Done, Children: tasks(Todo, Todo)},
	}}
	if got := deepPinned.EffectiveStatus(); got != Done {
		t.Errorf("recursive pinned-issue rollup = %q, want done", got)
	}
}

func TestValueHints(t *testing.T) {
	if got := StatusValues(); got != "todo|doing|done|paused" {
		t.Errorf("StatusValues = %q", got)
	}
	if got := PriorityValues(); got != "high|medium|low" {
		t.Errorf("PriorityValues = %q", got)
	}
}
