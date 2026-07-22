// Package server implements thingd's HTTP layer: a JSON API over the shared Go
// data layer, an SSE reload stream, and static serving of the built SPA (wired
// in a later commit). It is transport only — every read goes through
// internal/exporter and every write through internal/store, so the web and the
// CLI share identical semantics.
//
// Nodes are addressed by their ref (a slug-path like "epic/issue/task"), which
// is used verbatim as the URL path: /api/nodes/<ref>. Because a ref spans
// multiple path segments, per-field operations are carried in the PATCH body
// rather than as a path suffix; each PATCH carries exactly one operation.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

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

// Server serves the JSON API and the SPA. It is an http.Handler.
type Server struct {
	store  *store.Store
	static fs.FS
	now    func() string
	logger *log.Logger
	hub    *hub
	mux    *http.ServeMux
}

// New builds a Server over the given store.
func New(st *store.Store, opts Options) *Server {
	if opts.Now == nil {
		opts.Now = func() string { return time.Now().Format("2006-01-02") }
	}
	s := &Server{
		store:  st,
		static: opts.Static,
		now:    opts.Now,
		logger: opts.Logger,
		hub:    newHub(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tree", s.handleTree)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("POST /api/nodes/{parent...}", s.handleCreate)
	mux.HandleFunc("PATCH /api/nodes/{ref...}", s.handleUpdate)
	mux.HandleFunc("DELETE /api/nodes/{ref...}", s.handleRemove)
	mux.HandleFunc("/", s.handleStatic)
	s.mux = mux
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.logger != nil {
		s.logger.Printf("%s %s", r.Method, r.URL.Path)
	}
	rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	// A successful mutating request may have changed the tree; wake SSE clients
	// immediately rather than waiting for the filesystem poller to notice.
	if r.Method != http.MethodGet && r.Method != http.MethodHead &&
		rec.code >= 200 && rec.code < 300 {
		s.hub.broadcast()
	}
}

// statusRecorder captures the response status so ServeHTTP can tell whether a
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

func (s *Server) handleTree(w http.ResponseWriter, _ *http.Request) {
	data, err := exporter.Export(s.store)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(data); err != nil && s.logger != nil {
		s.logger.Printf("write /api/tree: %v", err)
	}
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
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
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
	ref, err := s.store.Add(parent, n)
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

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	e := s.locate(w, r)
	if e == nil {
		return
	}
	var req patchReq
	if !decode(w, r, &req) {
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
		if !model.Status(*req.Status).Valid() {
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
		if err := s.store.Save(e); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}

	if req.AddLink != nil {
		if strings.TrimSpace(req.AddLink.URL) == "" {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("a url is required"))
			return
		}
		if err := s.store.AddLink(e, req.AddLink.URL, req.AddLink.Label, s.now()); err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
	}
	if req.RemoveLink != nil {
		if err := s.store.RemoveLink(e, *req.RemoveLink, s.now()); err != nil {
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
		if err := s.store.Mv(e.Ref, dst, s.now()); err != nil {
			s.fail(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ref": dst})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ref": e.Ref})
}

func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	e := s.locate(w, r)
	if e == nil {
		return
	}
	if err := s.store.Remove(e); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// locate resolves the {ref...} path value, writing a 404 and returning nil when
// it does not exist.
func (s *Server) locate(w http.ResponseWriter, r *http.Request) *store.Entry {
	ref := r.PathValue("ref")
	e, err := s.store.Locate(ref)
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
// falling back to index.html for unknown paths so client-side routing works.
// (The embedded bundle is wired in a later commit; until then Static is nil and
// non-API paths 404. In development the Vite dev server serves the frontend and
// proxies /api and /events here, so this handler is not exercised.)
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
