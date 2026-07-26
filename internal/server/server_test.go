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
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/muniere/thing/internal/registry"
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
	return New([]Mount{{Name: "test", Store: fixture(t)}}, Options{Now: func() string { return "2026-07-20" }})
}

// proj returns the single project mounted by newServer, for tests that reach
// into the store or hub directly.
func proj(t *testing.T, s *Server) *project {
	t.Helper()
	p := s.project("test")
	if p == nil {
		t.Fatal(`no "test" project mounted`)
	}
	return p
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
	w := do(t, s, "GET", "/api/projects/test/tree", "")
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
	w := do(t, s, "POST", "/api/projects/test/nodes/alpha/one", `{"title":"New task"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "alpha/one/new-task" {
		t.Fatalf("ref = %q, want alpha/one/new-task", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/new-task"); e == nil {
		t.Fatal("created task not found in store")
	}
}

func TestCreateBadParent(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "POST", "/api/projects/test/nodes/nope", `{"title":"Orphan"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestCreateEpic(t *testing.T) {
	s := newServer(t)
	// An empty parent path creates a top-level epic.
	w := do(t, s, "POST", "/api/projects/test/nodes/", `{"title":"Beta","category":"Project"}`)
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
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"status":"done"}`); w.Code != http.StatusOK {
		t.Fatalf("status set = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	e, _ := proj(t, s).store.Locate("alpha/one/do-it")
	if e.Node.Status != "done" {
		t.Errorf("status = %q, want done", e.Node.Status)
	}
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"priority":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad priority = %d, want 400", w.Code)
	}
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"status":"bogus"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad status = %d, want 400", w.Code)
	}
}

func TestClearStatusRevertsToRollup(t *testing.T) {
	s := newServer(t)
	// Pin the issue's status, then clear it with an empty status: the explicit
	// value is dropped so it rolls up from its children again.
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one", `{"status":"paused"}`); w.Code != http.StatusOK {
		t.Fatalf("set = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e, _ := proj(t, s).store.Locate("alpha/one"); e.Node.Status != "paused" {
		t.Fatalf("status = %q, want paused", e.Node.Status)
	}
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one", `{"status":""}`); w.Code != http.StatusOK {
		t.Fatalf("clear = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	e, _ := proj(t, s).store.Locate("alpha/one")
	if e.Node.Status != "" {
		t.Errorf("status = %q, want empty (rolled up)", e.Node.Status)
	}
	// The child task is todo, so the reverted issue rolls up to todo.
	if got := e.Node.EffectiveStatus(); got != "todo" {
		t.Errorf("effective status = %q, want todo", got)
	}
}

func TestPatchSingleOperation(t *testing.T) {
	s := newServer(t)
	ref := "/api/projects/test/nodes/alpha/one/do-it"
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
	if w := do(t, s, "POST", "/api/projects/test/nodes/alpha/one", `{"title":"T","category":"X"}`); w.Code != http.StatusBadRequest {
		t.Errorf("create category on non-epic = %d, want 400", w.Code)
	}
	// On patch: setting a category on a non-epic node is 400.
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"category":"X"}`); w.Code != http.StatusBadRequest {
		t.Errorf("patch category on non-epic = %d, want 400", w.Code)
	}
}

func TestRenameKeepsRef(t *testing.T) {
	s := newServer(t)
	// A title change updates the frontmatter but keeps the slug (and thus ref)
	// stable, so links do not break.
	w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one/do-it", `{"title":"Renamed task"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "alpha/one/do-it" {
		t.Fatalf("ref = %q, want unchanged alpha/one/do-it", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); e == nil || e.Node.Title != "Renamed task" {
		t.Fatalf("title not updated: %+v", e)
	}
}

func TestMove(t *testing.T) {
	s := newServer(t)
	// Move the issue out of its epic into _orphan; its ref changes.
	w := do(t, s, "PATCH", "/api/projects/test/nodes/alpha/one", `{"move":"_orphan"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("move = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["ref"] != "_orphan/one" {
		t.Fatalf("ref = %q, want _orphan/one", res["ref"])
	}
	if e, _ := proj(t, s).store.Locate("_orphan/one"); e == nil {
		t.Fatal("moved issue not found at new ref")
	}
	// The whole subtree follows: the child task moved with its issue.
	if e, _ := proj(t, s).store.Locate("_orphan/one/do-it"); e == nil {
		t.Fatal("child task did not follow the move")
	}
	if e, _ := proj(t, s).store.Locate("alpha/one"); e != nil {
		t.Error("issue still present at the old ref")
	}
}

func TestBodyAndLinks(t *testing.T) {
	s := newServer(t)
	ref := "/api/projects/test/nodes/alpha/one/do-it"
	if w := do(t, s, "PATCH", ref, `{"body":"Fresh body."}`); w.Code != http.StatusOK {
		t.Fatalf("body = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w := do(t, s, "PATCH", ref, `{"addLink":{"url":"https://x.test","label":"X"}}`); w.Code != http.StatusOK {
		t.Fatalf("add link = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	e, _ := proj(t, s).store.Locate("alpha/one/do-it")
	if e.Node.Body != "Fresh body.\n" {
		t.Errorf("body = %q", e.Node.Body)
	}
	if len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://x.test" {
		t.Errorf("links = %v", e.Node.Links)
	}
	if w := do(t, s, "PATCH", ref, `{"removeLink":"https://x.test"}`); w.Code != http.StatusOK {
		t.Fatalf("rm link by url = %d, want 200", w.Code)
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); len(e.Node.Links) != 0 {
		t.Errorf("link not removed: %v", e.Node.Links)
	}
	// Removing by 1-based index also works; a bad index is a 400.
	do(t, s, "PATCH", ref, `{"addLink":{"url":"https://a.test"}}`)
	do(t, s, "PATCH", ref, `{"addLink":{"url":"https://b.test"}}`)
	if w := do(t, s, "PATCH", ref, `{"removeLink":"1"}`); w.Code != http.StatusOK {
		t.Fatalf("rm link by index = %d, want 200", w.Code)
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://b.test" {
		t.Errorf("index removal took the wrong link: %v", e.Node.Links)
	}
	if w := do(t, s, "PATCH", ref, `{"removeLink":"9"}`); w.Code != http.StatusBadRequest {
		t.Errorf("out-of-range index = %d, want 400", w.Code)
	}
}

func TestRemoveSubtree(t *testing.T) {
	s := newServer(t)
	// Removing an epic takes its whole subtree.
	if w := do(t, s, "DELETE", "/api/projects/test/nodes/alpha", ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove epic = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	for _, ref := range []string{"alpha", "alpha/one", "alpha/one/do-it"} {
		if e, _ := proj(t, s).store.Locate(ref); e != nil {
			t.Errorf("%q survived the subtree removal", ref)
		}
	}
}

func TestRemove(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "DELETE", "/api/projects/test/nodes/alpha/one/do-it", ""); w.Code != http.StatusNoContent {
		t.Fatalf("remove = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if e, _ := proj(t, s).store.Locate("alpha/one/do-it"); e != nil {
		t.Error("removed task still present")
	}
}

func TestNotFound(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PATCH", "/api/projects/test/nodes/nope", `{"status":"done"}`); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestConcurrentCreatesNoLostUpdate fires many creates under one parent at once,
// all with the same title so their slugs would collide. Serialized, the store
// deduplicates them (do-it-2, do-it-3, …) and every create survives; without the
// write lock the read-modify-write of the sibling set races and creates overwrite
// each other. It also runs clean under -race.
func TestConcurrentCreatesNoLostUpdate(t *testing.T) {
	s := newServer(t)
	const n = 24
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/projects/test/nodes/alpha/one", strings.NewReader(`{"title":"dup"}`))
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusCreated {
			t.Fatalf("create %d: status %d, want 201", i, c)
		}
	}

	// The issue started with one task (do-it); all n creates must be present too.
	type tnode struct {
		Ref      string  `json:"ref"`
		Children []tnode `json:"children"`
	}
	var tree []tnode
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/test/tree", "").Body.Bytes(), &tree); err != nil {
		t.Fatalf("tree not JSON: %v", err)
	}
	var count func(ns []tnode) int
	count = func(ns []tnode) int {
		for _, x := range ns {
			if x.Ref == "alpha/one" {
				return len(x.Children)
			}
			if c := count(x.Children); c >= 0 {
				return c
			}
		}
		return -1
	}
	if got := count(tree); got != n+1 {
		t.Fatalf("alpha/one has %d children, want %d (1 seed + %d creates) — lost updates", got, n+1, n)
	}
}

func TestStaticServingAndSPAFallback(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>thing</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{Static: static, Now: func() string { return "x" }})

	if w := do(t, s, "GET", "/assets/app.js", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "console.log") {
		t.Fatalf("asset serve = %d body=%q", w.Code, w.Body.String())
	}
	// Unknown client route falls back to the app shell.
	w := do(t, s, "GET", "/some/spa/route", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Fatalf("SPA fallback = %d body=%q", w.Code, w.Body.String())
	}
}

func TestConfigEndpoint(t *testing.T) {
	// No config.yaml -> the default title.
	s := newServer(t)
	w := do(t, s, "GET", "/api/projects/test/config", "")
	if w.Code != http.StatusOK {
		t.Fatalf("config = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["title"] != "thing" {
		t.Errorf("default title = %q, want thing", got["title"])
	}

	// A config.yaml title is served, along with the data dir path.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("title: My Board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := New([]Mount{{Name: "test", Store: store.Open(root)}}, Options{Now: func() string { return "x" }})
	w2 := do(t, s2, "GET", "/api/projects/test/config", "")
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if got["title"] != "My Board" {
		t.Errorf("title = %q, want My Board", got["title"])
	}
	if got["dir"] != root {
		t.Errorf("dir = %q, want %q", got["dir"], root)
	}
}

func TestNoStaticReturns404(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "GET", "/", ""); w.Code != http.StatusNotFound {
		t.Fatalf("no static root = %d, want 404", w.Code)
	}
}

func TestListProjects(t *testing.T) {
	// Two mounts: one with a config.yaml title, one falling back to its name.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("title: Work Board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New([]Mount{
		{Name: "work", Store: store.Open(root)},
		{Name: "home", Store: fixture(t)},
	}, Options{Now: func() string { return "x" }})

	w := do(t, s, "GET", "/api/projects", "")
	if w.Code != http.StatusOK {
		t.Fatalf("projects = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not JSON array: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2: %v", len(got), got)
	}
	// Registration order is preserved.
	if got[0]["name"] != "work" || got[0]["title"] != "Work Board" {
		t.Errorf("project[0] = %v, want work/Work Board", got[0])
	}
	if got[0]["dir"] != root {
		t.Errorf("project[0] dir = %q, want %q", got[0]["dir"], root)
	}
	// A project without a config title falls back to its name.
	if got[1]["name"] != "home" || got[1]["title"] != "home" {
		t.Errorf("project[1] = %v, want home/home", got[1])
	}
}

func TestUnknownProject404(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "GET", "/api/projects/nope/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown project tree = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if w := do(t, s, "POST", "/api/projects/nope/nodes/", `{"title":"X"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown project create = %d, want 404", w.Code)
	}
}

func TestListenExplicitFailsWhenTaken(t *testing.T) {
	// Occupy the port on the wildcard address, as another server would. Listen
	// binds the wildcard too, so this is what it must detect — a specific
	// 127.0.0.1 bind would not conflict with the wildcard on some platforms.
	held, err := net.Listen("tcp", ":0")
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

// thingTreeDir returns a temp directory that looks like an initialized thing
// tree (it holds a config.yaml), so Register accepts it.
func thingTreeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("title: Added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRegisterMountsAndLists(t *testing.T) {
	s := newServer(t)
	dir := thingTreeDir(t)

	w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+dir+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	// The new project answers on its own routes...
	if got := do(t, s, "GET", "/api/projects/added/tree", ""); got.Code != http.StatusOK {
		t.Fatalf("tree status = %d, want 200", got.Code)
	}
	// ...and shows up in the picker list alongside the original.
	list := do(t, s, "GET", "/api/projects", "")
	var items []map[string]string
	_ = json.Unmarshal(list.Body.Bytes(), &items)
	names := map[string]bool{}
	for _, it := range items {
		names[it["name"]] = true
	}
	if !names["test"] || !names["added"] {
		t.Fatalf("project list = %v, want both test and added", items)
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	s := newServer(t)
	dir := thingTreeDir(t)
	if w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+dir+`"}`); w.Code != http.StatusCreated {
		t.Fatalf("first PUT = %d, want 201", w.Code)
	}
	// Same name, same dir: a no-op that reports 200 rather than 201 or a conflict.
	if w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+dir+`"}`); w.Code != http.StatusOK {
		t.Fatalf("repeat PUT = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterConflictOnDifferentDir(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`); w.Code != http.StatusCreated {
		t.Fatalf("first PUT = %d, want 201", w.Code)
	}
	// Same name, a different dir: a conflict, not a silent re-point.
	if w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`); w.Code != http.StatusConflict {
		t.Fatalf("conflicting PUT = %d, want 409; body=%s", w.Code, w.Body.String())
	}
}

func TestRegisterRejectsBadName(t *testing.T) {
	s := newServer(t)
	// A non-slug name in the path (uppercase) is rejected.
	if w := do(t, s, "PUT", "/api/projects/NotASlug", `{"dir":"`+thingTreeDir(t)+`"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestRegisterRejectsNonThingDir(t *testing.T) {
	s := newServer(t)
	// A directory without config.yaml is not an initialized thing tree.
	if w := do(t, s, "PUT", "/api/projects/added", `{"dir":"`+t.TempDir()+`"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestUnregisterRemoves(t *testing.T) {
	s := newServer(t)
	dir := thingTreeDir(t)
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+dir+`"}`)

	if w := do(t, s, "DELETE", "/api/projects/added", ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	// Gone from routing...
	if w := do(t, s, "GET", "/api/projects/added/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("tree after delete = %d, want 404", w.Code)
	}
	// ...but the data directory is left on disk (unregister only).
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("unregister must not delete data dir: %v", err)
	}
}

func TestUnregisterUnknownIs404(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "DELETE", "/api/projects/nope", ""); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRegisterPersistsToRegistry(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	dir := thingTreeDir(t)

	// persist writes the whole registry, so the boot-mounted "test" is kept and
	// "added" is appended.
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+dir+`"}`)
	got, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load after register: %v", err)
	}
	if len(got) != 2 || got[1].Name != "added" || got[1].Dir != dir {
		t.Fatalf("registry after register = %+v, want test then {added %s}", got, dir)
	}

	// Unregister drops only "added"; "test" remains persisted.
	do(t, s, "DELETE", "/api/projects/added", "")
	got, err = registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load after unregister: %v", err)
	}
	if len(got) != 1 || got[0].Name != "test" {
		t.Fatalf("registry after unregister = %+v, want just test", got)
	}
}

func TestRegisterStartsWatcherUnregisterStops(t *testing.T) {
	s := newServer(t)
	go s.StartWatch(t.Context(), time.Hour) // long interval: we assert lifecycle, not polling
	// Let StartWatch capture the context before we register.
	waitFor(t, func() bool {
		s.regmu.RLock()
		defer s.regmu.RUnlock()
		return s.watchCtx != nil
	})

	dir := thingTreeDir(t)
	p, _, err := s.Register("added", dir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	s.regmu.RLock()
	watching := p.cancelWatch != nil
	s.regmu.RUnlock()
	if !watching {
		t.Fatal("Register should start the project's watcher")
	}

	if err := s.Unregister("added"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	s.regmu.RLock()
	stopped := p.cancelWatch == nil
	s.regmu.RUnlock()
	if !stopped {
		t.Fatal("Unregister should stop the project's watcher")
	}
}

// waitFor polls cond until true or a short deadline, for asserting on state a
// goroutine sets asynchronously.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}
