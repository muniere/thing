package exporter

import (
	"bytes"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// TestExportWebMatchesSchema validates ExportWeb output against schema/tree.json —
// the single source of truth the web types are generated from (see scripts/gen.sh).
// The schema sets additionalProperties:false, so a new Go field the schema (and
// thus the generated TS) does not know about fails here, catching Go↔schema drift
// that a green build would otherwise hide.
func TestExportWebMatchesSchema(t *testing.T) {
	sch := loadSchema(t)

	// A tree that exercises every field the schema describes.
	s := store.Open(t.TempDir())
	epic, _ := s.Add("", &model.Node{Title: "Web release", Category: "Project", Tags: []string{"release"}})
	issue, _ := s.Add(epic, &model.Node{
		Title:    "Monitor",
		Status:   "doing",
		Priority: "high",
		Body:     "Body text.",
		Links:    []model.Link{{URL: "https://x.test", Label: "X"}},
	})
	if _, err := s.Add(issue, &model.Node{Title: "Confirm"}); err != nil {
		t.Fatal(err)
	}

	data, err := ExportWeb(s)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// ExportWeb returns an array of nodes; the schema describes one node, and its
	// children recurse through the same schema, so validating each top-level node
	// covers the whole tree.
	nodes, ok := inst.([]any)
	if !ok {
		t.Fatalf("export is not a JSON array: %T", inst)
	}
	if len(nodes) == 0 {
		t.Fatal("export is empty")
	}
	for i, n := range nodes {
		if err := sch.Validate(n); err != nil {
			t.Fatalf("node %d fails schema: %v", i, err)
		}
	}
}

func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open("../../schema/tree.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("tree.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("tree.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}
