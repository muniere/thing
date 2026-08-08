package server

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/registry"
	"github.com/muniere/thing/internal/store"
	"gopkg.in/yaml.v3"
)

// TestMain insulates every test in this package from the developer's real
// global config (~/.config/thing/config.yaml, or wherever XDG_CONFIG_HOME
// points): it sets THING_CONFIG_DIR to a disposable directory before any test
// runs, so globalConfig() never reads a file this package does not control. A
// malformed real-world global filter block would otherwise turn into a 500 on
// every /api/.../config request in the suite. Tests that need a specific
// global config still set THING_CONFIG_DIR themselves via t.Setenv, which
// shadows this for their own duration.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "thing-server-test-config")
	if err != nil {
		panic(err)
	}
	os.Setenv("THING_CONFIG_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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
	mk(filepath.Join(root, "alpha", "one", "notes.html"), "<p>attachment</p>\n")
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

func TestGetNodeFile(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "GET", "/files/test/alpha/one/notes.html", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "<p>attachment</p>\n" {
		t.Errorf("body = %q", got)
	}
}

func TestGetNodeFileUnlistedIs404(t *testing.T) {
	s := newServer(t)
	// do-it.md is a task file, not a listed attachment, so it must not be
	// servable through the files route even though it exists on disk.
	w := do(t, s, "GET", "/files/test/alpha/one/do-it.md", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestGetNodeFileUnknownRefIs404(t *testing.T) {
	s := newServer(t)
	w := do(t, s, "GET", "/files/test/alpha/nope/notes.html", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
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
	// No global config in play: point THING_CONFIG_DIR at an empty directory so the
	// developer's real ~/.config/thing never leaks into the test.
	t.Setenv("THING_CONFIG_DIR", t.TempDir())

	// No config.yaml -> the default title, and no filter at all.
	s := newServer(t)
	w := do(t, s, "GET", "/api/projects/test/config", "")
	if w.Code != http.StatusOK {
		t.Fatalf("config = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got configRes
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "thing" {
		t.Errorf("default title = %q, want thing", got.Title)
	}
	if got.Filter != nil {
		t.Errorf("filter = %+v, want it omitted", got.Filter)
	}
	if strings.Contains(w.Body.String(), `"filter"`) {
		t.Errorf("body = %s, want no filter key", w.Body.String())
	}

	// A config.yaml title is served, along with the data dir path.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("title: My Board\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := New([]Mount{{Name: "test", Store: store.Open(root)}}, Options{Now: func() string { return "x" }})
	w2 := do(t, s2, "GET", "/api/projects/test/config", "")
	var got2 configRes
	if err := json.Unmarshal(w2.Body.Bytes(), &got2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got2.Title != "My Board" {
		t.Errorf("title = %q, want My Board", got2.Title)
	}
	if got2.Dir != root {
		t.Errorf("dir = %q, want %q", got2.Dir, root)
	}
}

// projects.yaml's top-level defaults supply the filter for every project, and a
// project's own entry overrides them key by key.
func TestConfigEndpointFilter(t *testing.T) {
	defaults := &registry.Defaults{Filter: parseFilter(t, "statuses: [todo, doing]\ntag: wip\n")}

	// Defaults only: the project writes no filter of its own.
	s := New([]Mount{{Name: "test", Store: fixture(t)}},
		Options{Defaults: defaults, Now: func() string { return "x" }})
	var got configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/test/config", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Filter == nil {
		t.Fatal("filter = nil, want the defaults")
	}
	if len(got.Filter.Statuses) != 2 || got.Filter.Statuses[0] != model.Todo {
		t.Errorf("statuses = %v, want [todo doing]", got.Filter.Statuses)
	}
	if got.Filter.Tag != "wip" {
		t.Errorf("tag = %q, want wip", got.Filter.Tag)
	}

	// The entry clears the inherited tag with an explicit null and keeps statuses.
	s2 := New([]Mount{{Name: "test", Store: fixture(t), Filter: parseFilter(t, "tag:\n")}},
		Options{Defaults: defaults, Now: func() string { return "x" }})
	body := do(t, s2, "GET", "/api/projects/test/config", "").Body.String()
	var got2 configRes
	if err := json.Unmarshal([]byte(body), &got2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got2.Filter.Statuses) != 2 {
		t.Errorf("statuses = %v, want the inherited [todo doing]", got2.Filter.Statuses)
	}
	if got2.Filter.Tag != "" {
		t.Errorf("tag = %q, want it cleared", got2.Filter.Tag)
	}
	// An empty facet is left out of the payload, matching projects.yaml's own
	// "an absent key means no filter".
	if strings.Contains(body, `"tag"`) {
		t.Errorf("body = %s, want no tag key", body)
	}
}

// With no defaults configured, a project's own entry stands on its own.
func TestConfigEndpointFilterProjectOnly(t *testing.T) {
	s := New([]Mount{{Name: "test", Store: fixture(t), Filter: parseFilter(t, "priorities: [high]\n")}},
		Options{Now: func() string { return "x" }})
	var got configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/test/config", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Filter == nil || len(got.Filter.Priorities) != 1 || got.Filter.Priorities[0] != model.High {
		t.Fatalf("filter = %+v, want priorities [high]", got.Filter)
	}
}

// parseFilter builds a registry.Filter from a YAML filter block, so tests state
// the same thing projects.yaml does — including presence, which decides whether a
// key inherits or clears.
func parseFilter(t *testing.T, body string) *registry.Filter {
	t.Helper()
	var f registry.Filter
	if err := yaml.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("parse filter %q: %v", body, err)
	}
	return &f
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
	if len(got.Projects) != 2 || got.Projects[1].Name != "added" || got.Projects[1].Dir != dir {
		t.Fatalf("registry after register = %+v, want test then {added %s}", got.Projects, dir)
	}

	// Unregister drops only "added"; "test" remains persisted.
	do(t, s, "DELETE", "/api/projects/added", "")
	got, err = registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load after unregister: %v", err)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "test" {
		t.Fatalf("registry after unregister = %+v, want just test", got.Projects)
	}
}

// pickerOrder returns the project names in the order GET /api/projects lists them.
func pickerOrder(t *testing.T, s *Server) []string {
	t.Helper()
	var items []map[string]string
	_ = json.Unmarshal(do(t, s, "GET", "/api/projects", "").Body.Bytes(), &items)
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it["name"]
	}
	return names
}

// registerThree mounts b and c alongside the fixture "test", giving order
// [test, b, c].
func registerThree(t *testing.T, s *Server) {
	t.Helper()
	do(t, s, "PUT", "/api/projects/b", `{"dir":"`+thingTreeDir(t)+`"}`)
	do(t, s, "PUT", "/api/projects/c", `{"dir":"`+thingTreeDir(t)+`"}`)
}

func TestMoveToFront(t *testing.T) {
	s := newServer(t)
	registerThree(t, s) // [test, b, c]
	// Front is "before the current first project" (test).
	if w := do(t, s, "PATCH", "/api/projects/c", `{"before":"test"}`); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"c", "test", "b"}) {
		t.Fatalf("order = %v, want [c test b]", got)
	}
}

func TestMoveAfter(t *testing.T) {
	s := newServer(t)
	registerThree(t, s) // [test, b, c]
	// Place test after b: [b, test, c].
	if w := do(t, s, "PATCH", "/api/projects/test", `{"after":"b"}`); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"b", "test", "c"}) {
		t.Fatalf("order = %v, want [b test c]", got)
	}
}

func TestMoveToEnd(t *testing.T) {
	s := newServer(t)
	registerThree(t, s) // [test, b, c]
	// End is "after the current last project" (c).
	if w := do(t, s, "PATCH", "/api/projects/test", `{"after":"c"}`); w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"b", "c", "test"}) {
		t.Fatalf("order = %v, want [b c test]", got)
	}
}

func TestMoveUnknownProjectIs404(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PATCH", "/api/projects/ghost", `{"after":"test"}`); w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestMoveBadAnchorIs400(t *testing.T) {
	s := newServer(t)
	registerThree(t, s)
	cases := map[string]string{
		"unknown anchor": `{"after":"ghost"}`,
		"anchor is self": `{"after":"test"}`,
		"neither given":  `{}`,
		"both given":     `{"before":"b","after":"c"}`,
	}
	for name, body := range cases {
		if w := do(t, s, "PATCH", "/api/projects/test", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body=%s", name, w.Code, w.Body.String())
		}
	}
}

func TestMovePersists(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`)
	do(t, s, "PATCH", "/api/projects/added", `{"before":"test"}`) // move added to front
	got, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Projects) != 2 || got.Projects[0].Name != "added" || got.Projects[1].Name != "test" {
		t.Fatalf("persisted order = %+v, want added then test", got.Projects)
	}
}

func TestReloadReconcilesFromFile(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	testDir := proj(t, s).store.Root
	bDir := thingTreeDir(t)

	// The file gains "b" alongside the boot-mounted "test": reload mounts it.
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "test", Dir: testDir}, {Name: "b", Dir: bDir}}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Reload()
	if err != nil {
		t.Fatalf("Reload add: %v", err)
	}
	if !slices.Equal(res.Added, []string{"b"}) || len(res.Removed) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("add result = %+v, want only Added [b]", res)
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"test", "b"}) {
		t.Fatalf("order after add = %v, want [test b]", got)
	}
	if w := do(t, s, "GET", "/api/projects/b/tree", ""); w.Code != http.StatusOK {
		t.Fatalf("b tree after add = %d, want 200", w.Code)
	}

	// The file reorders to [b, test]: reload matches the picker order to it.
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "b", Dir: bDir}, {Name: "test", Dir: testDir}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reload(); err != nil {
		t.Fatalf("Reload reorder: %v", err)
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"b", "test"}) {
		t.Fatalf("order after reorder = %v, want [b test]", got)
	}

	// The file drops "b": reload unmounts it, leaving only "test".
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "test", Dir: testDir}}}); err != nil {
		t.Fatal(err)
	}
	res, err = s.Reload()
	if err != nil {
		t.Fatalf("Reload remove: %v", err)
	}
	if !slices.Equal(res.Removed, []string{"b"}) {
		t.Fatalf("remove result = %+v, want Removed [b]", res)
	}
	if w := do(t, s, "GET", "/api/projects/b/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("b tree after remove = %d, want 404", w.Code)
	}
}

func TestReloadRepointsChangedDir(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	// A fresh thing tree with a distinct title proves the mount re-pointed.
	newDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(newDir, "config.yaml"), []byte("title: Repointed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "test", Dir: newDir}}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !slices.Equal(res.Repointed, []string{"test"}) {
		t.Fatalf("result = %+v, want Repointed [test]", res)
	}
	var cfg map[string]string
	_ = json.Unmarshal(do(t, s, "GET", "/api/projects/test/config", "").Body.Bytes(), &cfg)
	if cfg["title"] != "Repointed" {
		t.Fatalf("title = %q, want Repointed (mount did not re-point)", cfg["title"])
	}
}

func TestReloadSkipsBadDir(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	testDir := proj(t, s).store.Root
	badDir := t.TempDir() // no config.yaml → not a thing tree
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "test", Dir: testDir}, {Name: "bad", Dir: badDir}}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Name != "bad" {
		t.Fatalf("result = %+v, want one Skipped bad", res)
	}
	if len(res.Added) != 0 {
		t.Fatalf("a bad dir must not be mounted: %+v", res)
	}
	if w := do(t, s, "GET", "/api/projects/bad/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("bad tree = %d, want 404 (not mounted)", w.Code)
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"test"}) {
		t.Fatalf("order = %v, want [test]", got)
	}
}

func TestReloadEndpoint(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	testDir := proj(t, s).store.Root
	if err := registry.Save(regFile, &registry.Registry{Projects: []registry.Project{{Name: "test", Dir: testDir}, {Name: "b", Dir: thingTreeDir(t)}}}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s, "POST", "/api/projects/reload", "")
	if w.Code != http.StatusOK {
		t.Fatalf("reload = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var res struct{ Added []string }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if !slices.Equal(res.Added, []string{"b"}) {
		t.Fatalf("added = %v, want [b]", res.Added)
	}
}

func TestReloadNoRegistryFileIsNoop(t *testing.T) {
	s := newServer(t) // no RegistryFile configured
	w := do(t, s, "POST", "/api/projects/reload", "")
	if w.Code != http.StatusOK {
		t.Fatalf("reload without registry = %d, want 200 (no-op)", w.Code)
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"test"}) {
		t.Fatalf("order = %v, want [test] unchanged", got)
	}
}

func TestEditRenamesKeepingPosition(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	registerThree(t, s) // [test, b, c]

	if w := do(t, s, "PATCH", "/api/projects/b", `{"name":"mid"}`); w.Code != http.StatusNoContent {
		t.Fatalf("rename = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if w := do(t, s, "GET", "/api/projects/mid/tree", ""); w.Code != http.StatusOK {
		t.Fatalf("mid tree = %d, want 200", w.Code)
	}
	if w := do(t, s, "GET", "/api/projects/b/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("b tree after rename = %d, want 404", w.Code)
	}
	if got := pickerOrder(t, s); !slices.Equal(got, []string{"test", "mid", "c"}) {
		t.Fatalf("order = %v, want [test mid c]", got)
	}
	got, _ := registry.Load(regFile)
	names := make([]string, len(got.Projects))
	for i, p := range got.Projects {
		names[i] = p.Name
	}
	if !slices.Equal(names, []string{"test", "mid", "c"}) {
		t.Fatalf("persisted = %v, want [test mid c]", names)
	}
}

func TestEditRepointsDir(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Now:          func() string { return "2026-07-20" },
		RegistryFile: regFile,
	})
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`)
	// A fresh tree with a distinct title proves the mount re-pointed.
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "config.yaml"), []byte("title: Repointed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := do(t, s, "PATCH", "/api/projects/added", `{"dir":"`+dir2+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("repoint = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	var cfg map[string]string
	_ = json.Unmarshal(do(t, s, "GET", "/api/projects/added/config", "").Body.Bytes(), &cfg)
	if cfg["title"] != "Repointed" || cfg["dir"] != dir2 {
		t.Fatalf("config = %v, want title Repointed / dir %q", cfg, dir2)
	}
	got, _ := registry.Load(regFile)
	if len(got.Projects) != 2 || got.Projects[1].Name != "added" || got.Projects[1].Dir != dir2 {
		t.Fatalf("persisted = %+v, want added -> %q", got.Projects, dir2)
	}
}

func TestEditRenameAndRepoint(t *testing.T) {
	s := newServer(t)
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`)
	dir2 := thingTreeDir(t)
	if w := do(t, s, "PATCH", "/api/projects/added", `{"name":"renamed","dir":"`+dir2+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("edit = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if w := do(t, s, "GET", "/api/projects/renamed/tree", ""); w.Code != http.StatusOK {
		t.Fatalf("renamed tree = %d, want 200", w.Code)
	}
	if w := do(t, s, "GET", "/api/projects/added/tree", ""); w.Code != http.StatusNotFound {
		t.Fatalf("old name after edit = %d, want 404", w.Code)
	}
	var cfg map[string]string
	_ = json.Unmarshal(do(t, s, "GET", "/api/projects/renamed/config", "").Body.Bytes(), &cfg)
	if cfg["dir"] != dir2 {
		t.Fatalf("dir = %q, want %q", cfg["dir"], dir2)
	}
}

func TestEditErrors(t *testing.T) {
	s := newServer(t) // "test" is already mounted
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`)
	cases := []struct {
		name, target, body string
		code               int
	}{
		{"rename to taken name", "/api/projects/added", `{"name":"test"}`, http.StatusConflict},
		{"rename to bad slug", "/api/projects/added", `{"name":"Not A Slug"}`, http.StatusBadRequest},
		{"repoint to non-thing dir", "/api/projects/added", `{"dir":"` + t.TempDir() + `"}`, http.StatusBadRequest},
		{"empty name", "/api/projects/added", `{"name":""}`, http.StatusBadRequest},
		{"empty dir", "/api/projects/added", `{"dir":""}`, http.StatusBadRequest},
		{"unknown project", "/api/projects/ghost", `{"name":"x"}`, http.StatusNotFound},
		{"move and edit together", "/api/projects/added", `{"before":"test","name":"x"}`, http.StatusBadRequest},
		{"empty patch", "/api/projects/added", `{}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if w := do(t, s, "PATCH", c.target, c.body); w.Code != c.code {
			t.Errorf("%s: status = %d, want %d; body=%s", c.name, w.Code, c.code, w.Body.String())
		}
	}
}

func TestEditRepointRestartsWatcher(t *testing.T) {
	s := newServer(t)
	go s.StartWatch(t.Context(), time.Hour) // long interval: assert lifecycle, not polling
	waitFor(t, func() bool {
		s.regmu.RLock()
		defer s.regmu.RUnlock()
		return s.watchCtx != nil
	})

	old, _, err := s.Register("added", thingTreeDir(t))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := s.Edit("added", "", thingTreeDir(t), nil); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	s.regmu.RLock()
	oldStopped := old.cancelWatch == nil
	np := s.projects["added"]
	newWatching := np != nil && np != old && np.cancelWatch != nil
	s.regmu.RUnlock()
	if !oldStopped {
		t.Error("re-point should stop the old mount's watcher")
	}
	if !newWatching {
		t.Error("re-point should install a fresh project with a running watcher")
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
// Themes are served as one stylesheet per name from /themes/<name>.css, layering
// the built-in set with the reader's own directory so adding a theme takes no
// code change.
func TestThemeRoute(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("THING_DATA_DIR", stateDir)

	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Themes: fstest.MapFS{"teal.css": {Data: []byte(":root[data-theme=\"teal\"]{--bg:#0d1616}")}},
		Now:    func() string { return "x" },
	})

	w := do(t, s, "GET", "/themes/teal.css", "")
	if w.Code != http.StatusOK {
		t.Fatalf("teal.css = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("content-type = %q, want text/css", ct)
	}
	if !strings.Contains(w.Body.String(), "#0d1616") {
		t.Errorf("body = %q, want the built-in teal palette", w.Body.String())
	}

	// A theme only the reader's own directory defines is served like any other.
	themes := filepath.Join(stateDir, "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themes, "ocean.css"), []byte("/*ocean*/"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := do(t, s, "GET", "/themes/ocean.css", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/*ocean*/") {
		t.Fatalf("ocean.css = %d body=%q, want the user's own theme", w.Code, w.Body.String())
	}
}

// An unknown theme is a 404, not an error page: the board keeps its default
// palette, which is what a typo in config.yaml should come to.
func TestThemeRouteUnknown(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Themes: fstest.MapFS{"teal.css": {Data: []byte("/*teal*/")}},
		Static: fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}},
		Now:    func() string { return "x" },
	})
	for _, path := range []string{"/themes/nope.css", "/themes/teal", "/themes/Teal.css", "/themes/.css"} {
		if w := do(t, s, "GET", path, ""); w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (body=%q)", path, w.Code, w.Body.String())
		}
	}
	// A traversal never reaches the handler: ServeMux normalizes the path and
	// redirects, so the request lands somewhere else entirely. Whatever the
	// status, it must not come back as a stylesheet.
	w := do(t, s, "GET", "/themes/../secret.css", "")
	if strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") {
		t.Errorf("traversal served CSS: %d %q", w.Code, w.Body.String())
	}
}

// The theme is served alongside the filter so a board can color itself per
// project. It resolves project-entry first, falling back to the defaults.
func TestConfigEndpointTheme(t *testing.T) {
	// Defaults only.
	s := New([]Mount{{Name: "test", Store: fixture(t)}},
		Options{Defaults: &registry.Defaults{Theme: "slate"}, Now: func() string { return "x" }})
	var got configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/test/config", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Theme != "slate" {
		t.Errorf("theme = %q, want the default slate", got.Theme)
	}

	// The project's own entry wins.
	s2 := New([]Mount{{Name: "test", Store: fixture(t), Theme: "teal"}},
		Options{Defaults: &registry.Defaults{Theme: "slate"}, Now: func() string { return "x" }})
	var got2 configRes
	if err := json.Unmarshal(do(t, s2, "GET", "/api/projects/test/config", "").Body.Bytes(), &got2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got2.Theme != "teal" {
		t.Errorf("theme = %q, want the project's teal", got2.Theme)
	}
}

// With nothing configured anywhere the key is left out entirely, so the frontend
// keeps its default palette rather than being handed an empty theme name.
func TestConfigEndpointThemeUnset(t *testing.T) {
	body := do(t, newServer(t), "GET", "/api/projects/test/config", "").Body.String()
	if strings.Contains(body, `"theme"`) {
		t.Errorf("body = %s, want no theme key", body)
	}
}

// The picker needs the names of the themes that exist to offer them, and they
// come from the files rather than from a list in code.
func TestThemeListEndpoint(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Themes: fstest.MapFS{"teal.css": {Data: []byte("/*teal*/")}, "amber.css": {Data: []byte("")}},
		Now:    func() string { return "x" },
	})
	var got struct {
		Themes []string `json:"themes"`
	}
	w := do(t, s, "GET", "/api/themes", "")
	if w.Code != http.StatusOK {
		t.Fatalf("themes = %d, want 200", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !slices.Equal(got.Themes, []string{"amber", "teal"}) {
		t.Errorf("themes = %v, want [amber teal]", got.Themes)
	}
}

// The picker shows which theme each project is on, so the listing carries it.
func TestListProjectsCarriesTheme(t *testing.T) {
	s := New([]Mount{{Name: "test", Store: fixture(t), Theme: "teal"}, {Name: "plain", Store: fixture(t)}},
		Options{Now: func() string { return "x" }})
	var got []struct {
		Name  string `json:"name"`
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got[0].Theme != "teal" {
		t.Fatalf("projects = %+v, want test on teal", got)
	}
	if got[1].Theme != "" {
		t.Errorf("plain theme = %q, want empty", got[1].Theme)
	}
}

// Setting a theme from the picker writes it to the project's entry, so it
// survives a restart.
func TestPatchProjectTheme(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		RegistryFile: regFile,
		Now:          func() string { return "x" },
	})
	if w := do(t, s, "PATCH", "/api/projects/test", `{"theme":"teal"}`); w.Code != http.StatusNoContent {
		t.Fatalf("set theme = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	var cfg configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/test/config", "").Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Theme != "teal" {
		t.Errorf("theme = %q, want teal", cfg.Theme)
	}
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.Projects[0].Theme != "teal" {
		t.Errorf("persisted theme = %q, want teal", reg.Projects[0].Theme)
	}

	// An empty theme clears the entry, falling back to the registry defaults.
	if w := do(t, s, "PATCH", "/api/projects/test", `{"theme":""}`); w.Code != http.StatusNoContent {
		t.Fatalf("clear theme = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	reg, _ = registry.Load(regFile)
	if reg.Projects[0].Theme != "" {
		t.Errorf("persisted theme = %q, want it cleared", reg.Projects[0].Theme)
	}
}

func TestPatchProjectThemeRejects(t *testing.T) {
	s := newServer(t)
	// An unsafe name must never reach a data-theme attribute or a URL.
	if w := do(t, s, "PATCH", "/api/projects/test", `{"theme":"../evil"}`); w.Code != http.StatusBadRequest {
		t.Errorf("unsafe theme = %d, want 400", w.Code)
	}
	// A theme is its own operation, not combinable with a move or an edit.
	if w := do(t, s, "PATCH", "/api/projects/test", `{"theme":"teal","before":"other"}`); w.Code != http.StatusBadRequest {
		t.Errorf("theme + move = %d, want 400", w.Code)
	}
	if w := do(t, s, "PATCH", "/api/projects/nope", `{"theme":"teal"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown project = %d, want 404", w.Code)
	}
}

// A rename or re-point rebuilds the mount, so it has to carry the project's
// display settings across. Dropping them would silently reset a board's filter
// and palette on an unrelated edit.
func TestEditKeepsDisplaySettings(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t), Theme: "teal", Filter: parseFilter(t, "tag: api\n")}},
		Options{RegistryFile: regFile, Now: func() string { return "x" }})

	if w := do(t, s, "PATCH", "/api/projects/test", `{"name":"renamed"}`); w.Code != http.StatusNoContent {
		t.Fatalf("rename = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	var cfg configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/renamed/config", "").Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Theme != "teal" {
		t.Errorf("theme after rename = %q, want teal", cfg.Theme)
	}
	if cfg.Filter == nil || cfg.Filter.Tag != "api" {
		t.Errorf("filter after rename = %+v, want tag api", cfg.Filter)
	}
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.Projects[0].Theme != "teal" || reg.Projects[0].Filter == nil {
		t.Errorf("persisted = %+v, want the theme and filter kept", reg.Projects[0])
	}
}

// The theme rides along with a rename in one request, so the edit dialog can save
// everything it shows at once.
func TestEditNameAndTheme(t *testing.T) {
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{Now: func() string { return "x" }})
	if w := do(t, s, "PATCH", "/api/projects/test", `{"name":"renamed","theme":"violet"}`); w.Code != http.StatusNoContent {
		t.Fatalf("edit = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	var cfg configRes
	if err := json.Unmarshal(do(t, s, "GET", "/api/projects/renamed/config", "").Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Theme != "violet" {
		t.Errorf("theme = %q, want violet", cfg.Theme)
	}
}

// The color scheme is registry-wide rather than per project, so it has its own
// endpoint: the root picker needs it too, and the picker has no project.
func TestSettingsEndpoint(t *testing.T) {
	s := New([]Mount{{Name: "test", Store: fixture(t)}},
		Options{Defaults: &registry.Defaults{Scheme: "light"}, Now: func() string { return "x" }})
	var got struct {
		Scheme string `json:"scheme"`
	}
	w := do(t, s, "GET", "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("settings = %d, want 200", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Scheme != "light" {
		t.Errorf("scheme = %q, want light", got.Scheme)
	}
}

// Unset reads as "auto" rather than an empty string, so the client has one less
// special case to know about.
func TestSettingsEndpointUnset(t *testing.T) {
	var got struct {
		Scheme string `json:"scheme"`
	}
	if err := json.Unmarshal(do(t, newServer(t), "GET", "/api/settings", "").Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Scheme != "auto" {
		t.Errorf("scheme = %q, want auto", got.Scheme)
	}
}

func TestPatchSettings(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}},
		Options{RegistryFile: regFile, Now: func() string { return "x" }})

	if w := do(t, s, "PATCH", "/api/settings", `{"scheme":"dark"}`); w.Code != http.StatusNoContent {
		t.Fatalf("set = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.Defaults == nil || reg.Defaults.Scheme != "dark" {
		t.Errorf("persisted defaults = %+v, want scheme dark", reg.Defaults)
	}

	// "auto" clears it, so the file says nothing rather than saying "auto".
	if w := do(t, s, "PATCH", "/api/settings", `{"scheme":"auto"}`); w.Code != http.StatusNoContent {
		t.Fatalf("clear = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	reg, _ = registry.Load(regFile)
	if reg.Defaults != nil && reg.Defaults.Scheme != "" {
		t.Errorf("persisted scheme = %q, want it cleared", reg.Defaults.Scheme)
	}
}

func TestPatchSettingsRejects(t *testing.T) {
	s := newServer(t)
	if w := do(t, s, "PATCH", "/api/settings", `{"scheme":"bright"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad scheme = %d, want 400", w.Code)
	}
}

// A scheme set on the registry survives a register, which rewrites the file.
func TestSettingsSurviveRegister(t *testing.T) {
	regFile := filepath.Join(t.TempDir(), "projects.yaml")
	s := New([]Mount{{Name: "test", Store: fixture(t)}}, Options{
		Defaults:     &registry.Defaults{Scheme: "dark"},
		RegistryFile: regFile,
		Now:          func() string { return "x" },
	})
	do(t, s, "PUT", "/api/projects/added", `{"dir":"`+thingTreeDir(t)+`"}`)
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reg.Defaults == nil || reg.Defaults.Scheme != "dark" {
		t.Errorf("defaults after register = %+v, want scheme dark", reg.Defaults)
	}
}
