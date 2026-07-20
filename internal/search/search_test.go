package search

import (
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

func entry(ref, title, slug string, tags ...string) *store.Entry {
	return &store.Entry{Ref: ref, Node: &model.Node{Type: model.Task, Title: title, Slug: slug, Tags: tags}}
}

func TestFind(t *testing.T) {
	entries := []*store.Entry{
		entry("a/monitor", "Monitor rollout", "monitor", "observability"),
		entry("a/confirm", "Confirm routing", "confirm"),
		entry("a/web", "Web release", "web"),
	}

	// A contiguous substring match ranks first.
	if r := Find(entries, "monitor"); len(r) == 0 || r[0].Ref != "a/monitor" {
		t.Errorf("monitor: %+v", r)
	}
	// "rout" is a contiguous substring of "routing" (confirm) but only a
	// scattered subsequence of "rollout" (monitor), so confirm ranks higher.
	r := Find(entries, "rout")
	if len(r) != 2 || r[0].Ref != "a/confirm" || r[1].Ref != "a/monitor" {
		t.Errorf("rout ranking: %+v", r)
	}
	// A tag matches.
	if r := Find(entries, "observability"); len(r) == 0 || r[0].Ref != "a/monitor" {
		t.Errorf("tag: %+v", r)
	}
	// No match is excluded.
	if r := Find(entries, "zzzzz"); len(r) != 0 {
		t.Errorf("no-match: %+v", r)
	}
	// An empty query matches everything.
	if r := Find(entries, ""); len(r) != 3 {
		t.Errorf("empty query: %d results, want 3", len(r))
	}
	// A prefix match beats a non-prefix substring.
	if r := Find(entries, "web"); len(r) == 0 || r[0].Ref != "a/web" {
		t.Errorf("prefix: %+v", r)
	}
}

// A prefix match ranks above a mid-string substring of the same query.
func TestFindPrefixBeatsSubstring(t *testing.T) {
	entries := []*store.Entry{
		entry("a/mid", "New website", "mid"), // "web" at index 4
		entry("a/pre", "Web release", "pre"), // "web" at index 0 (prefix)
	}
	if r := Find(entries, "web"); len(r) != 2 || r[0].Ref != "a/pre" {
		t.Errorf("prefix should rank first: %+v", r)
	}
}

// A contiguous-run subsequence ranks above a scattered one.
func TestFindConsecutiveBeatsScattered(t *testing.T) {
	entries := []*store.Entry{
		entry("a/scatter", "a x b x c", "scatter"), // a,b,c scattered
		entry("a/run", "z abc z", "run"),           // abc contiguous (but not a substring path? it is)
	}
	// "abc" is a substring of "z abc z" -> substring path; of "a x b x c" only a
	// subsequence. The substring wins, which also confirms substring > subsequence.
	if r := Find(entries, "abc"); len(r) != 2 || r[0].Ref != "a/run" {
		t.Errorf("contiguous should rank first: %+v", r)
	}
}

// A real substring far past index 100 is still found, not silently dropped by a
// score underflow.
func TestFindLongTitleSubstring(t *testing.T) {
	long := strings.Repeat("x", 120) + "needle"
	entries := []*store.Entry{entry("a/long", long, "long")}
	if r := Find(entries, "needle"); len(r) != 1 || r[0].Score < 1 {
		t.Errorf("far substring dropped: %+v", r)
	}
}

// Multibyte (Japanese) queries match over runes, not raw bytes.
func TestFindMultibyte(t *testing.T) {
	entries := []*store.Entry{entry("a/jp", "設計ドキュメント", "jp")}
	if r := Find(entries, "ドキュメント"); len(r) != 1 {
		t.Errorf("multibyte substring: %+v", r)
	}
}

// Equal-scoring results are ordered by ref (stable and unique).
func TestFindTieBreakByRef(t *testing.T) {
	entries := []*store.Entry{
		entry("b/x", "Same", "x"),
		entry("a/x", "Same", "x"),
	}
	r := Find(entries, "same")
	if len(r) != 2 || r[0].Ref != "a/x" || r[1].Ref != "b/x" {
		t.Errorf("tie-break: %+v", r)
	}
}
