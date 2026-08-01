package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// PATCH nodes/<ref> {"archive": true} shelves the node and returns its archive
// ref; it drops out of the live tree.
func TestArchiveViaPatch(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"archive":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("archive = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "_archives/do-it" {
		t.Fatalf("ref = %q, want _archives/do-it", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); e != nil {
		t.Error("archived node still in the live tree")
	}
}

// GET /archives lists archived entries with their origin.
func TestArchiveListEndpoint(t *testing.T) {
	s := newServer(t)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"archive":true}`)

	w := do(t, s, "GET", "/api/projects/test/archives", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("body not a JSON array: %v", err)
	}
	if len(items) != 1 || items[0]["ref"] != "_archives/do-it" || items[0]["from"] != "alpha/one/do-it" {
		t.Fatalf("archive list = %v", items)
	}
}

// PATCH /archives/<name> restores the entry; with no body it lands on its origin.
func TestUnarchiveViaPatch(t *testing.T) {
	s := newServer(t)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"archive":true}`)

	w := do(t, s, "PATCH", "/api/projects/test/archives/do-it", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("unarchive = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "alpha/one/do-it" {
		t.Fatalf("ref = %q, want alpha/one/do-it", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); e == nil {
		t.Error("node not restored to the live tree")
	}
}

// An epic archives with its whole subtree and can be unarchived to a new ref.
func TestArchiveEpicAndUnarchiveTo(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha", `{"archive":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("archive epic = %d; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "_archives/alpha" {
		t.Fatalf("archived ref = %q, want _archives/alpha", res["ref"])
	}

	// Listed as an epic, from its origin ref.
	lw := do(t, s, "GET", "/api/projects/test/archives", "")
	var items []map[string]any
	_ = json.Unmarshal(lw.Body.Bytes(), &items)
	if len(items) != 1 || items[0]["type"] != "epic" || items[0]["from"] != "alpha" {
		t.Fatalf("archive list = %v", items)
	}

	// Restore under a new ref with --to; the whole subtree comes back.
	uw := do(t, s, "PATCH", "/api/projects/test/archives/alpha", `{"to":"beta"}`)
	if uw.Code != http.StatusOK {
		t.Fatalf("unarchive --to = %d; body=%s", uw.Code, uw.Body.String())
	}
	_ = json.Unmarshal(uw.Body.Bytes(), &res)
	if res["ref"] != "beta" {
		t.Fatalf("restored ref = %q, want beta", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("beta/one/do-it"); e == nil {
		t.Error("epic subtree not restored under the new ref")
	}
}

// GET /archives reports an archived node's own status, not one rolled up from
// children it never loaded: a statusless issue over a done task must not surface
// as "todo".
func TestArchiveListStatusIsOwnNotRolledUp(t *testing.T) {
	s := newServer(t)
	// Clear the issue's own status and finish its task, so a rollup would read
	// "done" but the issue itself carries no status.
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one", `{"status":""}`)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"status":"done"}`)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one", `{"archive":true}`)

	w := do(t, s, "GET", "/api/projects/test/archives", "")
	var items []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("body: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("archive list = %v", items)
	}
	// No own status, so the row carries none — not a bogus "todo" from rolling up
	// zero loaded children.
	if st, ok := items[0]["status"]; ok && st != "" {
		t.Errorf("archived issue status = %v, want absent (own status is empty)", st)
	}
}

// GET /archives/<name> returns the archived node's detail — where it came from,
// its own status, and its body — the web equivalent of `show _archives/<name>`.
func TestArchiveGetEndpoint(t *testing.T) {
	s := newServer(t)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"body":"hello body"}`)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"archive":true}`)

	w := do(t, s, "GET", "/api/projects/test/archives/do-it", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get = %d; body=%s", w.Code, w.Body.String())
	}
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	body, _ := res["body"].(string)
	if res["ref"] != "_archives/do-it" || res["from"] != "alpha/one/do-it" || !strings.Contains(body, "hello body") {
		t.Fatalf("archive detail = %v", res)
	}

	// An unknown archive name is a 404.
	if w := do(t, s, "GET", "/api/projects/test/archives/ghost", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown = %d, want 404", w.Code)
	}
}

// Restoring onto an occupied origin fails; an unknown archive name is a 404.
func TestUnarchiveErrors(t *testing.T) {
	s := newServer(t)
	do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"archive":true}`)
	// Recreate a node at the origin.
	do(t, s, "POST", "/api/projects/test/nodes/alpha/one", `{"title":"Do it"}`)

	if w := do(t, s, "PATCH", "/api/projects/test/archives/do-it", `{}`); w.Code == http.StatusOK {
		t.Errorf("unarchive onto occupied origin = %d, want an error", w.Code)
	}
	if w := do(t, s, "PATCH", "/api/projects/test/archives/ghost", `{}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown archive name = %d, want 404", w.Code)
	}
}
