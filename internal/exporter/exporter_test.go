package exporter

import (
	"encoding/json"
	"testing"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

func TestExportOrphanRef(t *testing.T) {
	s := store.Open(t.TempDir())
	// A top-level orphan issue is addressed under _orphan/, and its task nests.
	iss, err := s.Add(store.OrphanDir, &model.Node{Title: "Loose"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(iss, &model.Node{Title: "Do"}); err != nil {
		t.Fatal(err)
	}
	data, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	_ = json.Unmarshal(data, &got)
	if len(got) != 1 || got[0].Ref != "_orphan/loose" {
		t.Fatalf("orphan issue ref = %q, want _orphan/loose", got[0].Ref)
	}
	if len(got[0].Children) != 1 || got[0].Children[0].Ref != "_orphan/loose/do" {
		t.Fatalf("orphan task ref = %+v", got[0].Children)
	}
}

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
	if e.Ref != "web-release" || e.Category != "Project" {
		t.Errorf("epic fields: %+v", e)
	}
	// Children nest, and each carries its full slug-path ref.
	if len(e.Children) != 1 || len(e.Children[0].Children) != 1 {
		t.Fatalf("nesting lost: %+v", e)
	}
	if e.Children[0].Ref != "web-release/monitor" {
		t.Errorf("issue ref = %q", e.Children[0].Ref)
	}
	task := e.Children[0].Children[0]
	if task.Priority != model.Priority("high") || task.Ref != "web-release/monitor/confirm" {
		t.Errorf("task fields: %+v", task)
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

func TestExportCarriesOwnStatusOnly(t *testing.T) {
	s := store.Open(t.TempDir())
	epic, _ := s.Add("", &model.Node{Title: "E"})
	// The statusless epic rolls up from its done issue, but Export is the
	// interchange format: it carries only the stored own status, so the epic has
	// none and never picks up the derived rollup.
	if _, err := s.Add(epic, &model.Node{Title: "I", Status: "done"}); err != nil {
		t.Fatal(err)
	}

	data, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	_ = json.Unmarshal(data, &got)
	if got[0].Status != "" {
		t.Errorf("rolled-up epic own status = %q, want empty", got[0].Status)
	}
	if got[0].Children[0].Status != model.Status("done") {
		t.Errorf("child own status = %q, want done", got[0].Children[0].Status)
	}
	// The interchange format never carries the derived status, on any node.
	var raw []map[string]any
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw[0]["status"]; ok {
		t.Error("rolled-up epic should omit status entirely")
	}
	if _, ok := raw[0]["effectiveStatus"]; ok {
		t.Error("Export must not carry effectiveStatus")
	}
}

func TestExportWebAddsEffectiveStatus(t *testing.T) {
	s := store.Open(t.TempDir())
	epic, _ := s.Add("", &model.Node{Title: "E"})
	if _, err := s.Add(epic, &model.Node{Title: "I", Status: "done"}); err != nil {
		t.Fatal(err)
	}

	data, err := ExportWeb(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	_ = json.Unmarshal(data, &got)
	// The rolled-up epic keeps no own status but exposes the derived one for the
	// client to display.
	if got[0].Status != "" {
		t.Errorf("epic own status = %q, want empty", got[0].Status)
	}
	if got[0].EffectiveStatus != model.Status("done") {
		t.Errorf("epic effectiveStatus = %q, want done", got[0].EffectiveStatus)
	}
	// The pinned child carries both, equal to each other.
	if c := got[0].Children[0]; c.Status != "done" || c.EffectiveStatus != "done" {
		t.Errorf("child status=%q effectiveStatus=%q, want both done", c.Status, c.EffectiveStatus)
	}
}

func TestMarkersAreWebOnly(t *testing.T) {
	s := store.Open(t.TempDir())
	// Markers is a read-time derivation like effectiveStatus and files:
	// `thing import` has no way to consume it, so it belongs to the web shape
	// only, not the interchange format. This body is missing its Definition
	// of Done section, so ExportWeb should report exactly one warning.
	body := "## Summary\n\nOne line.\n\n## Details\n\nMore text.\n"
	if _, err := s.Add("", &model.Node{Title: "E", Body: body}); err != nil {
		t.Fatal(err)
	}

	exportData, err := Export(s)
	if err != nil {
		t.Fatal(err)
	}
	var got []node
	if err := json.Unmarshal(exportData, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got[0].Markers) != 0 {
		t.Errorf("Export markers = %+v, want empty", got[0].Markers)
	}
	var raw []map[string]any
	if err := json.Unmarshal(exportData, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw[0]["markers"]; ok {
		t.Error("Export must not carry markers")
	}
	if _, ok := raw[0]["effectiveStatus"]; ok {
		t.Error("Export must not carry effectiveStatus")
	}
	if _, ok := raw[0]["files"]; ok {
		t.Error("Export must not carry files")
	}

	webData, err := ExportWeb(s)
	if err != nil {
		t.Fatal(err)
	}
	var gotWeb []node
	if err := json.Unmarshal(webData, &gotWeb); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	markers := gotWeb[0].Markers
	if len(markers) != 1 || markers[0].Severity != "warn" || markers[0].Message != "No Definition of Done section" {
		t.Errorf("ExportWeb markers = %+v, want a single missing-Definition-of-Done warning", markers)
	}
}
