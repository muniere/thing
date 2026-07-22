package server

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/muniere/thing/internal/store"
)

func fixture(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	mk := func(p, c string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join(root, "alpha", "_epic.md"), "---\ntitle: Alpha\ncategory: Project\n---\n")
	mk(filepath.Join(root, "alpha", "one", "_issue.md"), "---\ntitle: One\nstatus: todo\n---\nIssue body.\n")
	mk(filepath.Join(root, "alpha", "one", "do-it.md"), "---\ntitle: Do it\nstatus: todo\n---\n")
	return store.Open(root)
}

func newServer(t *testing.T) *Server {
	t.Helper()
	return New(fixture(t), Options{Now: func() string { return "2026-07-20" }})
}

func do(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, r)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

func TestGetTree(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "GET", "/api/tree", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var nodes []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("body not JSON array: %v", err)
	}
	if len(nodes) != 1 || nodes[0]["ref"] != "alpha" {
		t.Fatalf("tree = %v, want single epic alpha with ref", nodes)
	}
	if _, ok := nodes[0]["slug"]; ok {
		t.Error("export should not emit a separate slug (derivable from ref)")
	}
}

func TestCreateTask(t *testing.T) {
	s := newServer(t)
	// The parent issue's ref is in the path; the parent decides the type.
	w := do(t, s, "POST", "/api/nodes/alpha/one", `{"title":"New task"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "alpha/one/new-task" {
		t.Fatalf("ref = %q, want alpha/one/new-task", res["ref"])
	}
	if e, _ := s.store.Locate("alpha/one/new-task"); e == nil {
		t.Fatal("created task not found in store")
	}
}

func TestCreateBadParent(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "POST", "/api/nodes/nope", `{"title":"Orphan"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestCreateEpic(t *testing.T) {
	s := newServer(t)
	// An empty parent path creates a top-level epic.
	w := do(t, s, "POST", "/api/nodes/", `{"title":"Beta","category":"Project"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "beta" {
		t.Fatalf("ref = %q, want beta", res["ref"])
	}
}

func TestStatusAndPriority(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"status":"done"}`); w.Code != http.StatusOK {
		t.Fatalf("status set = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	e, _ := s.store.Locate("alpha/one/do-it")
	if e.Node.Status != "done" {
		t.Errorf("status = %q, want done", e.Node.Status)
	}
	if w := do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"priority":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad priority = %d, want 400", w.Code)
	}
	if w := do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"status":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad status = %d, want 400", w.Code)
	}
}

func TestPatchSingleOperation(t *testing.T) {
	s := newServer(t)
	ref := "/api/nodes/alpha/one/do-it"
	// A body with no recognized field is a 400, not a silent 200 no-op.
	if w := do(t, s, "PATCH", ref, `{}`); w.Code != http.StatusBadRequest {
		t.Errorf("empty patch = %d, want 400", w.Code)
	}
	// An unknown field is rejected rather than silently dropped.
	if w := do(t, s, "PATCH", ref, `{"titel":"typo"}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", w.Code)
	}
	// Mixing two operations is rejected so a PATCH stays atomic.
	if w := do(t, s, "PATCH", ref, `{"status":"done","move":"_orphan"}`); w.Code != http.StatusBadRequest {
		t.Errorf("multi-op patch = %d, want 400", w.Code)
	}
}

func TestCategoryRejectedOnNonEpic(t *testing.T) {
	s := newServer(t)
	// On create: a category with a non-empty parent (i.e. not an epic) is 400.
	if w := do(t, s, "POST", "/api/nodes/alpha/one", `{"title":"T","category":"X"}`); w.Code != http.StatusBadRequest {
		t.Errorf("create category on non-epic = %d, want 400", w.Code)
	}
	// On patch: setting a category on a non-epic node is 400.
	if w := do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"category":"X"}`); w.Code != http.StatusBadRequest {
		t.Errorf("patch category on non-epic = %d, want 400", w.Code)
	}
}

func TestRenameKeepsRef(t *testing.T) {
	s := newServer(t)
	// A title change updates the frontmatter but keeps the slug (and thus ref)
	// stable, so links do not break.
	w := do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"title":"Renamed task"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "alpha/one/do-it" {
		t.Fatalf("ref = %q, want unchanged alpha/one/do-it", res["ref"])
	}
	if e, _ := s.store.Locate("alpha/one/do-it"); e == nil || e.Node.Title != "Renamed task" {
		t.Fatalf("title not updated: %+v", e)
	}
}

func TestMove(t *testing.T) {
	s := newServer(t)
	// Move the issue out of its epic into _orphan; its ref changes.
	w := do(t, s, "PATCH", "/api/nodes/alpha/one", `{"move":"_orphan"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("move = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "_orphan/one" {
		t.Fatalf("ref = %q, want _orphan/one", res["ref"])
	}
	if e, _ := s.store.Locate("_orphan/one"); e == nil {
		t.Fatal("moved issue not found at new ref")
	}
	// The whole subtree follows: the child task moved with its issue.
	if e, _ := s.store.Locate("_orphan/one/do-it"); e == nil {
		t.Fatal("child task did not follow the move")
	}
	if e, _ := s.store.Locate("alpha/one"); e != nil {
		t.Error("issue still present at the old ref")
	}
}

func TestBodyAndLinks(t *testing.T) {
	s := newServer(t)
	ref := "/api/nodes/alpha/one/do-it"
	if w := do(t, s, "PATCH", ref, `{"body":"Fresh body."}`); w.Code != http.StatusOK {
		t.Fatalf("body = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w := do(t, s, "PATCH", ref, `{"addLink":{"url":"https://x.test","label":"X"}}`); w.Code != http.StatusOK {
		t.Fatalf("add link = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	e, _ := s.store.Locate("alpha/one/do-it")
	if e.Node.Body != "Fresh body.\n" {
		t.Errorf("body = %q", e.Node.Body)
	}
	if len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://x.test" {
		t.Errorf("links = %v", e.Node.Links)
	}
	if w := do(t, s, "PATCH", ref, `{"removeLink":"https://x.test"}`); w.Code != http.StatusOK {
		t.Fatalf("rm link by url = %d, want 200", w.Code)
	}
	if e, _ := s.store.Locate("alpha/one/do-it"); len(e.Node.Links) != 0 {
		t.Errorf("link not removed: %v", e.Node.Links)
	}
	// Removing by 1-based index also works; a bad index is a 400.
	do(t, s, "PATCH", ref, `{"addLink":{"url":"https://a.test"}}`)
	do(t, s, "PATCH", ref, `{"addLink":{"url":"https://b.test"}}`)
	if w := do(t, s, "PATCH", ref, `{"removeLink":"1"}`); w.Code != http.StatusOK {
		t.Fatalf("rm link by index = %d, want 200", w.Code)
	}
	if e, _ := s.store.Locate("alpha/one/do-it"); len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://b.test" {
		t.Errorf("index removal took the wrong link: %v", e.Node.Links)
	}
	if w := do(t, s, "PATCH", ref, `{"removeLink":"9"}`); w.Code != http.StatusBadRequest {
		t.Errorf("out-of-range index = %d, want 400", w.Code)
	}
}

func TestRemoveSubtree(t *testing.T) {
	s := newServer(t)
	// Removing an epic takes its whole subtree.
	if w := do(t, s, "DELETE", "/api/nodes/alpha", ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove epic = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	for _, ref := range []string{"alpha", "alpha/one", "alpha/one/do-it"} {
		if e, _ := s.store.Locate(ref); e != nil {
			t.Errorf("%q survived the subtree removal", ref)
		}
	}
}

func TestRemove(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "DELETE", "/api/nodes/alpha/one/do-it", ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if e, _ := s.store.Locate("alpha/one/do-it"); e != nil {
		t.Error("removed task still present")
	}
}

func TestNotFound(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PATCH", "/api/nodes/nope", `{"status":"done"}`); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestStaticServingAndSPAFallback(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>thing</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := New(fixture(t), Options{Static: static, Now: func() string { return "x" }})

	if w := do(t, s, "GET", "/assets/app.js", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "console.log") {
		t.Fatalf("asset serve = %d body=%q", w.Code, w.Body.String())
	}
	// Unknown client route falls back to the app shell.
	w := do(t, s, "GET", "/some/spa/route", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Fatalf("SPA fallback = %d body=%q", w.Code, w.Body.String())
	}
}

func TestNoStaticReturns404(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "GET", "/", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no static root = %d, want 404", w.Code)
	}
}

func TestListenExplicitFailsWhenTaken(t *testing.T) {
	// Occupy a port, then demand it explicitly.
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	port := held.Addr().(*net.TCPAddr).Port

	if _, err := Listen(port, true); err == nil {
		t.Fatal("explicit Listen on a taken port should fail")
	}
	// Non-explicit falls back to a free port above it.
	ln, err := Listen(port, false)
	if err != nil {
		t.Fatalf("fallback Listen failed: %v", err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr).Port; got == port {
		t.Fatalf("fallback reused the taken port %d", got)
	}
}
