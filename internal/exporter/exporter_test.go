package exporter

import (
	"encoding/json"
	"testing"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

func TestExportNestsAndOmitsEmpties(t *testing.T) {
	s := store.Open(t.TempDir())
	epic, err := s.Add("", &model.Node{Title: "Web release", Category: "Project"})
	if err != nil {
		t.Fatal(err)
	}
	issue, err := s.Add(epic, &model.Node{Title: "Monitor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(issue, &model.Node{Title: "Confirm", Priority: "high"}); err != nil {
		t.Fatal(err)
	}

	data, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 top node, got %d", len(got))
	}
	e := got[0]
	if e.Slug != "web-release" || e.Category != "Project" {
		t.Errorf("epic fields: %+v", e)
	}
	// Children nest.
	if len(e.Children) != 1 || len(e.Children[0].Children) != 1 {
		t.Fatalf("nesting lost: %+v", e)
	}
	task := e.Children[0].Children[0]
	if task.Priority != model.Priority("high") {
		t.Errorf("task priority: %+v", task)
	}

	// Empty optional fields are omitted from the raw JSON (no priority on the
	// epic, no tags/links/body anywhere).
	var raw []map[string]any
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw[0]["priority"]; ok {
		t.Error("empty priority should be omitted")
	}
	if _, ok := raw[0]["tags"]; ok {
		t.Error("empty tags should be omitted")
	}
}

func TestExportUsesEffectiveStatus(t *testing.T) {
	s := store.Open(t.TempDir())
	epic, _ := s.Add("", &model.Node{Title: "E"})
	// The statusless epic rolls up from its issues: its only issue is done, so
	// the epic exports as done even though its own file sets no status.
	if _, err := s.Add(epic, &model.Node{Title: "I", Status: "done"}); err != nil {
		t.Fatal(err)
	}

	data, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	_ = json.Unmarshal(data, &got)
	if got[0].Status != model.Status("done") {
		t.Errorf("epic effective status = %q, want done", got[0].Status)
	}
}
