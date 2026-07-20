package importer

import (
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// run is a test helper that imports a JSON literal into a fresh store.
func run(t *testing.T, s *store.Store, batch string, dryRun bool) ([]Result, bool) {
	t.Helper()
	results, ok, err := Run(s, []byte(batch), dryRun, "2026-07-21")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return results, ok
}

func refs(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Ref
	}
	return out
}

func TestImportHierarchy(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[
		{"type":"epic","title":"Infra"},
		{"type":"issue","title":"Provision","parent":"infra"},
		{"type":"task","title":"Terraform apply","parent":"infra/provision","priority":"high","tags":["ops"],
		 "body":"run it","links":[{"url":"https://example.com","label":"doc"}]}
	]`, false)

	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	want := []string{"infra", "infra/provision", "infra/provision/terraform-apply"}
	for i, w := range want {
		if results[i].Ref != w || results[i].Status != StatusCreated {
			t.Errorf("item %d = %+v, want ref %q created", i, results[i], w)
		}
	}
	// The batch-created parents resolve on disk, and every field carries through.
	idx, _ := s.Index()
	e := idx["infra/provision/terraform-apply"]
	if e == nil {
		t.Fatal("task not written")
	}
	n := e.Node
	// The body round-trips through the on-disk file, which ends with a newline.
	if n.Priority != model.Priority("high") || len(n.Tags) != 1 || strings.TrimSpace(n.Body) != "run it" {
		t.Errorf("scalar fields not carried: %+v", n)
	}
	if len(n.Links) != 1 || n.Links[0].URL != "https://example.com" || n.Links[0].Label != "doc" {
		t.Errorf("links not carried: %+v", n.Links)
	}
}

func TestImportEpicCategory(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[{"type":"epic","title":"Platform","category":"Project"}]`, false)
	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	idx, _ := s.Index()
	if e := idx["platform"]; e == nil || e.Node.Category != "Project" {
		t.Errorf("category not carried: %+v", e)
	}
}

func TestImportOrphanAndInbox(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[
		{"type":"issue","title":"Standalone"},
		{"title":"Note one","parent":"inbox"},
		{"title":"Note two","parent":"inbox"}
	]`, false)

	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	if results[0].Ref != "_orphan/standalone" {
		t.Errorf("orphan issue ref = %q", results[0].Ref)
	}
	// Both inbox tasks land under a single auto-created inbox issue.
	if results[1].Ref != "_orphan/inbox/note-one" || results[2].Ref != "_orphan/inbox/note-two" {
		t.Errorf("inbox refs = %q, %q", results[1].Ref, results[2].Ref)
	}
	idx, _ := s.Index()
	if e := idx["_orphan/inbox"]; e == nil || e.Node.Type != model.Issue {
		t.Errorf("inbox issue not created once: %+v", e)
	}
}

func TestImportInboxReusesPreexisting(t *testing.T) {
	s := store.Open(t.TempDir())
	// An inbox issue already on disk from a prior run is reused, not duplicated.
	if _, err := s.Add(store.OrphanDir, &model.Node{Title: "Inbox"}); err != nil {
		t.Fatal(err)
	}
	results, ok := run(t, s, `[{"title":"Late note","parent":"inbox"}]`, false)
	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	if results[0].Ref != "_orphan/inbox/late-note" {
		t.Errorf("ref = %q, want under pre-existing inbox", results[0].Ref)
	}
	// Still exactly one inbox issue (no inbox-2).
	idx, _ := s.Index()
	if _, dup := idx["_orphan/inbox-2"]; dup {
		t.Error("a second inbox issue was created")
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[
		{"type":"epic","title":"Infra"},
		{"type":"issue","title":"Provision","parent":"infra"}
	]`, true)

	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	if results[1].Ref != "infra/provision" || results[1].Status != StatusValidated {
		t.Errorf("dry-run should predict refs and report validated: %+v", results[1])
	}
	idx, _ := s.Index()
	if len(idx) != 0 {
		t.Errorf("dry-run wrote %d nodes, want 0", len(idx))
	}
}

// TestImportDryRunMatchesReal is the key guard: dry-run's in-memory slug/ref
// prediction must equal what a real import writes, even with slug collisions
// and the inbox. The two use independent code paths (in-memory mirror vs
// store.Add reading disk), so a divergence would be a real bug.
func TestImportDryRunMatchesReal(t *testing.T) {
	batch := `[
		{"type":"epic","title":"Dup"},
		{"type":"epic","title":"Dup"},
		{"type":"issue","title":"Work","parent":"dup"},
		{"title":"Note","parent":"inbox"},
		{"title":"Note","parent":"inbox"}
	]`
	dry, dryOK := run(t, store.Open(t.TempDir()), batch, true)
	real, realOK := run(t, store.Open(t.TempDir()), batch, false)

	if !dryOK || !realOK {
		t.Fatalf("both runs should succeed: dry=%v real=%v", dryOK, realOK)
	}
	dr, rr := refs(dry), refs(real)
	if strings.Join(dr, ",") != strings.Join(rr, ",") {
		t.Errorf("dry-run refs != real refs:\n dry:  %v\n real: %v", dr, rr)
	}
	// Sanity: the collisions actually happened (dup-2, note-2 under inbox).
	if rr[1] != "dup-2" || rr[4] != "_orphan/inbox/note-2" {
		t.Errorf("expected collisions to dedup: %v", rr)
	}
}

func TestImportPerItemErrors(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[
		{"type":"epic","title":"Good"},
		{"type":"issue","title":"Real issue","parent":"good"},
		{"title":""},
		{"type":"milestone","title":"Bad type"},
		{"type":"task","title":"Missing parent","parent":"nope"},
		{"type":"epic","title":"Epic with parent","parent":"good"},
		{"type":"task","title":"Parent is an epic","parent":"good"},
		{"type":"issue","title":"Parent is an issue","parent":"good/real-issue"},
		{"type":"task","title":"No parent at all"},
		{"type":"task","title":"Bad priority","parent":"inbox","priority":"urgent"},
		{"type":"epic","title":"Bad link","links":[{"label":"no url"}]}
	]`, false)

	if ok {
		t.Fatal("expected failure")
	}
	// index -> substring the error message must contain
	msgs := map[int]string{
		2:  "title is required",
		3:  `invalid type "milestone"`,
		4:  `no such issue "nope"`,
		5:  "an epic takes no parent",
		6:  `no such issue "good"`,           // exists, but it is an epic not an issue
		7:  `no such epic "good/real-issue"`, // exists, but it is an issue not an epic
		8:  "a task requires a parent issue",
		9:  `invalid priority "urgent"`,
		10: "link url is required",
	}
	for i, want := range msgs {
		if results[i].Status != StatusError {
			t.Errorf("item %d: expected error, got %+v", i, results[i])
			continue
		}
		if !strings.Contains(results[i].Message, want) {
			t.Errorf("item %d message = %q, want to contain %q", i, results[i].Message, want)
		}
	}
	// The good items still landed despite surrounding failures.
	if results[0].Status != StatusCreated || results[1].Status != StatusCreated {
		t.Errorf("good items did not land: %+v, %+v", results[0], results[1])
	}
}

func TestImportDedupesSiblingSlugs(t *testing.T) {
	s := store.Open(t.TempDir())
	results, ok := run(t, s, `[
		{"type":"epic","title":"Dup"},
		{"type":"epic","title":"Dup"}
	]`, false)

	if !ok {
		t.Fatalf("expected success: %+v", results)
	}
	if results[0].Ref != "dup" || results[1].Ref != "dup-2" {
		t.Errorf("sibling dedup = %q, %q; want dup, dup-2", results[0].Ref, results[1].Ref)
	}
	// Both landed on disk.
	idx, _ := s.Index()
	if idx["dup"] == nil || idx["dup-2"] == nil {
		t.Errorf("both epics should exist on disk: %v", refs(results))
	}
}

func TestImportMalformedJSON(t *testing.T) {
	_, ok, err := Run(store.Open(t.TempDir()), []byte(`{not an array`), false, "2026-07-21")
	if err == nil {
		t.Fatal("expected a hard error for malformed JSON")
	}
	if ok {
		t.Error("ok should be false on a hard error")
	}
}
