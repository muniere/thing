// Package server implements thingd's HTTP layer: a JSON API over the shared Go
// data layer, an SSE reload stream, and static serving of the embedded SPA. It
// is transport only — every read goes through
// internal/exporter and every write through internal/store, so the web and the
// CLI share identical semantics.
//
// A single server hosts multiple projects, each a named mount over its own data
// directory. Project routes nest under /api/projects/<project>/: .../tree,
// .../nodes/<ref>, .../events, while GET /api/projects lists the mounts for the
// root picker. Within a project, nodes are addressed by their ref (a slug-path
// like "epic/issue/task"), used verbatim as the URL path. Because a ref spans
// multiple path segments, per-field operations are carried in the PATCH body
// rather than as a path suffix; each PATCH carries exactly one operation.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muniere/thing/internal/config"
	"github.com/muniere/thing/internal/exporter"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// Options configures a Server.
type Options struct {
	Static fs.FS         // built SPA assets; nil disables static serving
	Now    func() string // today's date stamp for write timestamps; defaults to time.Now
	Logger *log.Logger   // access log; nil disables it
}

// Mount names a project and the store backing it. New takes an ordered list so
// the root picker can list projects in registration order.
type Mount struct {
	Name  string
	Store *store.Store
}

// project is one mounted project: its store plus a dedicated SSE hub so a change
// in one project reloads only that project's browsers, and its own lock so
// mutations serialize per-project rather than across the whole server.
type project struct {
	name  string
	store *store.Store
	hub   *hub
	// mu serializes store access across concurrent HTTP requests. The store reads
	// and writes the data dir directly, so without it two in-flight mutations (a
	// double-submit, a second tab, two nodes racing on the same slug) could
	// interleave their read-modify-write and clobber each other, and a read could
	// observe a half-written tree. Writes take Lock (exclusive, held across the
	// whole locate → mutate → save so it is atomic); the tree read takes RLock.
	// It does not coordinate with a separate CLI process touching the same dir.
	mu sync.RWMutex
}

// Server serves the JSON API and the SPA. It is an http.Handler.
type Server struct {
	static fs.FS
	now    func() string
	logger *log.Logger
	mux    *http.ServeMux
	// regmu guards the project registry (projects/order). A read takes RLock; a
	// dynamic register/unregister (a later phase) takes Lock.
	regmu    sync.RWMutex
	projects map[string]*project
	order    []string // registration order, for the root picker
	// bootID is a random nonce minted when the Server is constructed (once per
	// process in the real binary) and sent in the SSE hello frame. A client that
	// reconnects and sees a different bootID knows the server was replaced (a new
	// binary — e.g. air rebuilt it in dev) and reloads to pick up the new assets;
	// a reconnect to the same process keeps the same id and only refetches. It is
	// not persistence, just "is this the process I last talked to".
	bootID string
}

// New builds a Server over the given project mounts.
func New(mounts []Mount, opts Options) *Server {
	if opts.Now == nil {
		opts.Now = func() string { return time.Now().Format("2006-01-02") }
	}
	s := &Server{
		static:   opts.Static,
		now:      opts.Now,
		logger:   opts.Logger,
		projects: make(map[string]*project, len(mounts)),
		bootID:   newBootID(),
	}
	for _, m := range mounts {
		s.projects[m.Name] = &project{name: m.Name, store: m.Store, hub: newHub()}
		s.order = append(s.order, m.Name)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/projects/{project}/tree", s.withProject(s.handleTree))
	mux.HandleFunc("GET /api/projects/{project}/config", s.withProject(s.handleConfig))
	mux.HandleFunc("GET /api/projects/{project}/events", s.withProject(s.handleEvents))
	mux.HandleFunc("POST /api/projects/{project}/nodes/{parent...}", s.withProject(s.handleCreate))
	mux.HandleFunc("PATCH /api/projects/{project}/nodes/{ref...}", s.withProject(s.handleUpdate))
	mux.HandleFunc("DELETE /api/projects/{project}/nodes/{ref...}", s.withProject(s.handleRemove))
	mux.HandleFunc("/", s.handleStatic)
	s.mux = mux
	return s
}

// project returns the mount for name, or nil if none is registered.
func (s *Server) project(name string) *project {
	s.regmu.RLock()
	defer s.regmu.RUnlock()
	return s.projects[name]
}

// withProject resolves the {project} path segment, 404s when it is unknown, and
// otherwise dispatches to fn with the resolved project. It also broadcasts a
// reload to that project's hub after a successful mutation, so a change wakes
// only that project's SSE clients — reads and other projects are untouched.
func (s *Server) withProject(fn func(*project, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := s.project(r.PathValue("project"))
		if p == nil {
			s.fail(w, http.StatusNotFound, fmt.Errorf("no such project %q", r.PathValue("project")))
			return
		}
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		fn(p, rec, r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead &&
			rec.code >= 200 && rec.code < 300 {
			p.hub.broadcast()
		}
	}
}

// handleProjects lists the mounts for the root picker: each project's name, its
// display title (config.yaml title, else the name), and its data directory.
func (s *Server) handleProjects(w http.ResponseWriter, _ *http.Request) {
	s.regmu.RLock()
	list := make([]*project, 0, len(s.order))
	for _, name := range s.order {
		list = append(list, s.projects[name])
	}
	s.regmu.RUnlock()

	type item struct {
		Name  string `json:"name"`
		Title string `json:"title"`
		Dir   string `json:"dir"`
	}
	out := make([]item, 0, len(list))
	for _, p := range list {
		p.mu.RLock()
		root := p.store.Root
		cfg, err := config.Load(root)
		p.mu.RUnlock()
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		title := cfg.Title
		if title == "" {
			title = p.name
		}
		dir := root
		if abs, err := filepath.Abs(root); err == nil {
			dir = abs
		}
		out = append(out, item{Name: p.name, Title: title, Dir: dir})
	}
	writeJSON(w, http.StatusOK, out)
}

// newBootID mints the per-Server nonce sent in the SSE hello frame. crypto/rand
// makes it distinct per construction independent of the wall clock, so restart
// detection never turns on clock resolution (two builds relaunched back-to-back
// still get different ids).
func newBootID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand is effectively infallible; if it ever isn't, a timestamp
		// still yields a near-unique id rather than an empty one.
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.logger != nil {
		s.logger.Printf("%s %s", r.Method, r.URL.Path)
	}
	// Per-project mutation broadcasts happen in withProject, which knows which
	// project changed; ServeHTTP just logs and routes.
	s.mux.ServeHTTP(w, r)
}

// statusRecorder captures the response status so withProject can tell whether a
// mutation succeeded. It forwards Flush so SSE streaming still works.
type statusRecorder struct {
	http.ResponseWriter
	code    int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.code = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// --- reads ---

func (s *Server) handleTree(p *project, w http.ResponseWriter, _ *http.Request) {
	// Snapshot under a read lock, then release before writing the (possibly large)
	// body to a slow client so a reader never holds the lock across network I/O.
	// ExportWeb adds each node's effectiveStatus for the client to display.
	p.mu.RLock()
	data, err := exporter.ExportWeb(p.store)
	p.mu.RUnlock()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(data); err != nil && s.logger != nil {
		s.logger.Printf("write /api/tree: %v", err)
	}
}

// handleConfig serves the display config the web UI reads: the title (from
// config.yaml in the served data directory — the project-local .thing/ holds it
// alongside the nodes) and the data directory path itself, shown as a label. A
// missing or title-less config yields the default "thing", so the endpoint always
// returns a usable title.
func (s *Server) handleConfig(p *project, w http.ResponseWriter, _ *http.Request) {
	p.mu.RLock()
	root := p.store.Root
	cfg, err := config.Load(root)
	p.mu.RUnlock()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	title := cfg.Title
	if title == "" {
		title = "thing"
	}
	dir := root
	if abs, err := filepath.Abs(root); err == nil {
		dir = abs
	}
	writeJSON(w, http.StatusOK, map[string]string{"title": title, "dir": dir})
}

// --- writes ---

type createReq struct {
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"` // epics only
}

// handleCreate adds a child under the parent ref in the path. The parent decides
// the type, like the CLI's `add`: "" (empty path) → epic, "_orphan" → orphan
// issue, an epic ref → issue, an issue ref → task.
func (s *Server) handleCreate(p *project, w http.ResponseWriter, r *http.Request) {
	parent := r.PathValue("parent")
	var req createReq
	if !decode(w, r, &req) {
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("a title is required"))
		return
	}
	if req.Priority != "" && !model.Priority(req.Priority).Valid() {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("invalid priority %q", req.Priority))
		return
	}
	if req.Category != "" && parent != "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("category applies only to an epic"))
		return
	}
	n := &model.Node{
		Title:    title,
		Priority: model.Priority(req.Priority),
		Category: req.Category,
		Tags:     req.Tags,
		Updated:  s.now(),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	ref, err := p.store.Add(parent, n)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ref": ref})
}

// patchReq carries a single-field update. The web sends one operation per
// request; every present field is applied, and a move returns the new ref.
type patchReq struct {
	Status     *string  `json:"status"`
	Priority   *string  `json:"priority"`
	Title      *string  `json:"title"`
	Category   *string  `json:"category"`
	Body       *string  `json:"body"`
	Move       *string  `json:"move"` // new parent ref
	AddLink    *linkReq `json:"addLink"`
	RemoveLink *string  `json:"removeLink"` // url or 1-based index
}

type linkReq struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

func (s *Server) handleUpdate(p *project, w http.ResponseWriter, r *http.Request) {
	var req patchReq
	if !decode(w, r, &req) {
		return
	}
	// Hold the write lock across the whole locate → mutate → save/move so the
	// read-modify-write is atomic against any other in-flight mutation.
	p.mu.Lock()
	defer p.mu.Unlock()
	e := s.locate(p, w, r)
	if e == nil {
		return
	}

	// Exactly one operation per request. Frontmatter field sets count as one
	// (they are batched into a single Save); a link op or a move is another.
	// This keeps every PATCH atomic — there is no partial-write window across
	// independent Save/AddLink/RemoveLink/Mv steps.
	frontmatter := req.Status != nil || req.Priority != nil || req.Title != nil ||
		req.Category != nil || req.Body != nil
	ops := 0
	for _, present := range []bool{frontmatter, req.AddLink != nil, req.RemoveLink != nil, req.Move != nil} {
		if present {
			ops++
		}
	}
	if ops == 0 {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("no recognized field to update"))
		return
	}
	if ops > 1 {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("only one operation per request"))
		return
	}

	// Frontmatter field sets are batched into one Save.
	dirty := false
	if req.Status != nil {
		// An empty status clears the explicit value so the node reverts to its
		// child rollup (see model.Node.EffectiveStatus); any other value must be
		// valid.
		if *req.Status != "" && !model.Status(*req.Status).Valid() {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("invalid status %q", *req.Status))
			return
		}
		e.Node.Status = model.Status(*req.Status)
		dirty = true
	}
	if req.Priority != nil {
		if *req.Priority != "" && !model.Priority(*req.Priority).Valid() {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("invalid priority %q", *req.Priority))
			return
		}
		e.Node.Priority = model.Priority(*req.Priority)
		dirty = true
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("a title is required"))
			return
		}
		e.Node.Title = title
		dirty = true
	}
	if req.Category != nil {
		if *req.Category != "" && e.Node.Type != model.Epic {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("category applies only to an epic"))
			return
		}
		e.Node.Category = *req.Category
		dirty = true
	}
	if req.Body != nil {
		e.Node.Body = *req.Body
		dirty = true
	}
	if dirty {
		e.Node.Updated = s.now()
		if err := p.store.Save(e); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}

	if req.AddLink != nil {
		if strings.TrimSpace(req.AddLink.URL) == "" {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("a url is required"))
			return
		}
		if err := p.store.AddLink(e, req.AddLink.URL, req.AddLink.Label, s.now()); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	if req.RemoveLink != nil {
		if err := p.store.RemoveLink(e, *req.RemoveLink, s.now()); err != nil {
			s.fail(w, http.StatusBadRequest, err)
			return
		}
	}

	// A move changes the node's ref; do it last and report the new ref.
	if req.Move != nil {
		dst := e.Node.Slug
		if *req.Move != "" {
			dst = *req.Move + "/" + e.Node.Slug
		}
		if err := p.store.Mv(e.Ref, dst, s.now()); err != nil {
			s.fail(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ref": dst})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ref": e.Ref})
}

func (s *Server) handleRemove(p *project, w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := s.locate(p, w, r)
	if e == nil {
		return
	}
	if err := p.store.Remove(e); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// locate resolves the {ref...} path value, writing a 404 and returning nil when
// it does not exist.
func (s *Server) locate(p *project, w http.ResponseWriter, r *http.Request) *store.Entry {
	ref := r.PathValue("ref")
	e, err := p.store.Locate(ref)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return nil
	}
	if e == nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("no such node %q", ref))
		return nil
	}
	return e
}

func (s *Server) fail(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	// Reject unknown fields so a client typo (e.g. {"titel":...}) is a 400 rather
	// than a silently dropped no-op.
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// handleStatic serves the embedded SPA for any path not handled by the API,
// falling back to index.html for unknown paths so client-side routing works. The
// real binary always sets Static (thingd embeds web/dist); Static is nil only in
// tests that construct a Server without assets, where non-API paths 404.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.static == nil {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.static, name)
	if err != nil {
		// A genuine read error (corrupt bundle, permissions) is a 500, not an
		// SPA route. Only a missing file falls back to the app shell.
		if !errors.Is(err, fs.ErrNotExist) {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		name = "index.html"
		data, err = fs.ReadFile(s.static, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	_, _ = w.Write(data)
}
