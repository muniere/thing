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
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muniere/thing/internal/config"
	"github.com/muniere/thing/internal/exporter"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/registry"
	"github.com/muniere/thing/internal/slug"
	"github.com/muniere/thing/internal/store"
)

// Options configures a Server.
type Options struct {
	Static   fs.FS         // built SPA assets; nil disables static serving
	Now      func() string // today's date stamp for write timestamps; defaults to time.Now
	NowStamp func() string // RFC3339 instant for the archive time; defaults to time.Now
	Logger   *log.Logger   // access log; nil disables it
	// RegistryFile is projects.yaml's path. Dynamic register/unregister write the
	// updated registry back here so it survives a restart. Empty disables
	// persistence (the in-memory registry still mutates) — used in tests.
	RegistryFile string
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
	// cancelWatch stops this project's filesystem watcher. It is set when the
	// watcher starts (at boot for initial projects, at Register time for dynamic
	// ones) and called by Unregister so a removed project stops polling. nil means
	// no watcher is running yet. Guarded by the Server's regmu.
	cancelWatch context.CancelFunc
}

// Server serves the JSON API and the SPA. It is an http.Handler.
type Server struct {
	static   fs.FS
	now      func() string
	nowStamp func() string
	logger   *log.Logger
	mux      *http.ServeMux
	// regmu guards the project registry (projects/order). A read takes RLock; a
	// dynamic register/unregister (a later phase) takes Lock.
	regmu    sync.RWMutex
	projects map[string]*project
	order    []string // registration order, for the root picker
	regFile  string   // projects.yaml path for persistence; "" disables it
	// watchCtx/watchInterval are captured by StartWatch so a project registered
	// after boot can spin up its own watcher under the same lifetime. watchCtx is
	// nil until StartWatch runs (so Register before watching just skips it).
	watchCtx      context.Context
	watchInterval time.Duration
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
	if opts.NowStamp == nil {
		opts.NowStamp = func() string { return time.Now().Format(time.RFC3339) }
	}
	s := &Server{
		static:   opts.Static,
		now:      opts.Now,
		nowStamp: opts.NowStamp,
		logger:   opts.Logger,
		regFile:  opts.RegistryFile,
		projects: make(map[string]*project, len(mounts)),
		bootID:   newBootID(),
	}
	for _, m := range mounts {
		s.projects[m.Name] = &project{name: m.Name, store: m.Store, hub: newHub()}
		s.order = append(s.order, m.Name)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("POST /api/projects/reload", s.handleReload)
	mux.HandleFunc("PUT /api/projects/{project}", s.handleRegister)
	mux.HandleFunc("PATCH /api/projects/{project}", s.handlePatchProject)
	mux.HandleFunc("DELETE /api/projects/{project}", s.handleUnregister)
	mux.HandleFunc("GET /api/projects/{project}/tree", s.withProject(s.handleTree))
	mux.HandleFunc("GET /api/projects/{project}/config", s.withProject(s.handleConfig))
	mux.HandleFunc("GET /api/projects/{project}/archives", s.withProject(s.handleArchiveList))
	mux.HandleFunc("GET /api/projects/{project}/archives/{name}", s.withProject(s.handleArchive))
	mux.HandleFunc("GET /api/projects/{project}/events", s.withProject(s.handleEvents))
	mux.HandleFunc("POST /api/projects/{project}/nodes/{parent...}", s.withProject(s.handleCreate))
	mux.HandleFunc("PATCH /api/projects/{project}/nodes/{ref...}", s.withProject(s.handleUpdate))
	mux.HandleFunc("DELETE /api/projects/{project}/nodes/{ref...}", s.withProject(s.handleRemove))
	mux.HandleFunc("PATCH /api/projects/{project}/archives/{name}", s.withProject(s.handleUnarchive))
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

// httpError carries an HTTP status alongside a message so Register/Unregister can
// tell the handler which code to return (400 bad name/dir, 409 duplicate, 404
// unknown) without the handler re-deriving it.
type httpError struct {
	code int
	err  error
}

func (e *httpError) Error() string { return e.err.Error() }

// Register mounts a project at name over dir: it validates both, adds the mount,
// starts its watcher, and persists the updated registry. dir must already be an
// initialized thing tree — the server never creates it; use `thing init` first.
//
// It is an idempotent upsert, matching PUT semantics: registering a name that
// already points at the same dir is a no-op that reports created=false, so a
// repeated request is safe. A name already bound to a different dir is a conflict
// rather than a silent re-point — detach it with Unregister first. A bad name or
// a non-thing directory fails, leaving the server untouched.
func (s *Server) Register(name, dir string) (p *project, created bool, err error) {
	if name == "" || slug.Slugify(name) != name {
		return nil, false, &httpError{http.StatusBadRequest, fmt.Errorf("invalid project name %q: must be a URL-safe slug", name)}
	}
	if !isThingTree(dir) {
		return nil, false, &httpError{http.StatusBadRequest, fmt.Errorf("%q is not a thing project (no %s)", dir, config.FileName)}
	}

	s.regmu.Lock()
	defer s.regmu.Unlock()
	if existing, ok := s.projects[name]; ok {
		if filepath.Clean(existing.store.Root) == filepath.Clean(dir) {
			return existing, false, nil // idempotent: same name, same dir
		}
		return nil, false, &httpError{http.StatusConflict, fmt.Errorf("project %q is already registered to a different directory", name)}
	}

	p = s.mountLocked(name, dir)
	if err := s.persistLocked(); err != nil {
		s.unmountLocked(name) // undo the mount so disk and memory stay in step
		return nil, false, &httpError{http.StatusInternalServerError, fmt.Errorf("persist registry: %w", err)}
	}
	return p, true, nil
}

// Unregister removes a project from the registry: it stops the watcher, drops the
// mount, and persists. It does not touch the data directory — the tree stays on
// disk and can be re-registered later. It fails on an unknown name.
func (s *Server) Unregister(name string) error {
	s.regmu.Lock()
	defer s.regmu.Unlock()
	if _, ok := s.projects[name]; !ok {
		return &httpError{http.StatusNotFound, fmt.Errorf("no such project %q", name)}
	}
	s.unmountLocked(name)
	if err := s.persistLocked(); err != nil {
		return &httpError{http.StatusInternalServerError, fmt.Errorf("persist registry: %w", err)}
	}
	return nil
}

// SkippedProject is one registry entry Reload could not mount, with the reason.
type SkippedProject struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ReloadResult summarizes what a Reload changed, for the caller to surface.
type ReloadResult struct {
	Added     []string         `json:"added"`
	Removed   []string         `json:"removed"`
	Repointed []string         `json:"repointed"`
	Skipped   []SkippedProject `json:"skipped"`
}

// Reload re-reads projects.yaml and reconciles the in-memory registry to it, so a
// hand-edit to the file (or a change by another process) takes effect without a
// restart: entries new to the file are mounted, entries dropped from it are
// unmounted, a changed directory is re-pointed, and the picker order is matched to
// the file.
//
// The file stays the source of truth — Reload never writes it back. An entry whose
// directory is not a thing tree is therefore skipped and reported (left in the
// file rather than deleted), and an existing mount is kept when the file now points
// it at a bad directory. It fails only when the file itself cannot be read.
func (s *Server) Reload() (*ReloadResult, error) {
	if s.regFile == "" {
		// No registry file (as in tests): nothing to resync from, so reload is a
		// no-op — mirroring persistLocked's treatment of an unset registry.
		return &ReloadResult{Added: []string{}, Removed: []string{}, Repointed: []string{}, Skipped: []SkippedProject{}}, nil
	}
	desired, err := registry.Load(s.regFile)
	if err != nil {
		return nil, &httpError{http.StatusInternalServerError, fmt.Errorf("read registry: %w", err)}
	}

	s.regmu.Lock()
	defer s.regmu.Unlock()

	res := &ReloadResult{Added: []string{}, Removed: []string{}, Repointed: []string{}, Skipped: []SkippedProject{}}
	want := make(map[string]bool, len(desired))
	for _, p := range desired {
		want[p.Name] = true
	}

	// Drop projects the file no longer lists. Snapshot the order first, since
	// unmountLocked mutates it as it goes.
	for _, name := range append([]string(nil), s.order...) {
		if !want[name] {
			s.unmountLocked(name)
			res.Removed = append(res.Removed, name)
		}
	}

	// Mount new entries and re-point changed ones. A bad directory is skipped and
	// reported; an existing mount survives a file that now points it somewhere bad.
	for _, p := range desired {
		existing, mounted := s.projects[p.Name]
		if mounted && filepath.Clean(existing.store.Root) == filepath.Clean(p.Dir) {
			continue // unchanged
		}
		if !isThingTree(p.Dir) {
			res.Skipped = append(res.Skipped, SkippedProject{p.Name, fmt.Sprintf("%q is not a thing project (no %s)", p.Dir, config.FileName)})
			continue
		}
		if mounted {
			// Re-point: wake the current browsers once so they refetch, then replace
			// the mount so the refetch resolves the new directory under the same name.
			existing.hub.broadcast()
			s.unmountLocked(p.Name)
			s.mountLocked(p.Name, p.Dir)
			res.Repointed = append(res.Repointed, p.Name)
		} else {
			s.mountLocked(p.Name, p.Dir)
			res.Added = append(res.Added, p.Name)
		}
	}

	// Match the picker order to the file, keeping only names that ended up mounted
	// (a skipped entry has no mount).
	order := make([]string, 0, len(s.projects))
	for _, p := range desired {
		if _, ok := s.projects[p.Name]; ok {
			order = append(order, p.Name)
		}
	}
	s.order = order

	return res, nil
}

// Move changes one project's place in the picker order relative to another,
// stable identifier rather than a positional index: it places name immediately
// before or after the anchor project. Exactly one of before/after must be given —
// the other is empty. The front is "before the current first project", the end is
// "after the current last", so both ends are reachable without depending on
// numeric positions. It only reorders — mounts, stores, and watchers are
// untouched — and persists the new order so it survives a restart. It fails on an
// unknown project (404 for name in the path) or a bad anchor (400: not exactly one
// given, unknown, or equal to name).
func (s *Server) Move(name, before, after string) error {
	s.regmu.Lock()
	defer s.regmu.Unlock()

	if _, ok := s.projects[name]; !ok {
		return &httpError{http.StatusNotFound, fmt.Errorf("no such project %q", name)}
	}
	if (before == "") == (after == "") {
		return &httpError{http.StatusBadRequest, fmt.Errorf("specify exactly one of before/after")}
	}
	anchor := before
	if after != "" {
		anchor = after
	}
	if anchor == name {
		return &httpError{http.StatusBadRequest, fmt.Errorf("cannot move project %q relative to itself", name)}
	}
	if _, ok := s.projects[anchor]; !ok {
		return &httpError{http.StatusBadRequest, fmt.Errorf("no such anchor project %q", anchor)}
	}

	// Pull name out, find the anchor in what remains, and reinsert name just
	// before or just after it.
	rest := make([]string, 0, len(s.order)-1)
	for _, n := range s.order {
		if n != name {
			rest = append(rest, n)
		}
	}
	at := slices.Index(rest, anchor)
	if after != "" {
		at++
	}
	next := slices.Insert(rest, at, name)

	prev := s.order
	s.order = next
	if err := s.persistLocked(); err != nil {
		s.order = prev // keep disk and memory in step
		return &httpError{http.StatusInternalServerError, fmt.Errorf("persist registry: %w", err)}
	}
	return nil
}

// Edit renames a project and/or re-points it at a new directory, persisting the
// change. An empty newName keeps the current name; an empty newDir keeps the
// current directory. A rename requires newName to be an unused URL-safe slug; a
// re-point requires newDir to be an initialized thing tree.
//
// It replaces the mount rather than mutating the live one in place, so p.name and
// p.store stay immutable for the lock-free readers in handleProjects. A re-point
// keeps the URL, so it wakes the project's browsers to refetch the new tree; a
// rename changes the URL, so those browsers navigate away on their own.
func (s *Server) Edit(name, newName, newDir string) error {
	if newName == "" {
		newName = name
	}
	if newName != name && slug.Slugify(newName) != newName {
		return &httpError{http.StatusBadRequest, fmt.Errorf("invalid project name %q: must be a URL-safe slug", newName)}
	}
	if newDir != "" && !isThingTree(newDir) {
		return &httpError{http.StatusBadRequest, fmt.Errorf("%q is not a thing project (no %s)", newDir, config.FileName)}
	}

	s.regmu.Lock()
	defer s.regmu.Unlock()

	p, ok := s.projects[name]
	if !ok {
		return &httpError{http.StatusNotFound, fmt.Errorf("no such project %q", name)}
	}
	if newName != name {
		if _, taken := s.projects[newName]; taken {
			return &httpError{http.StatusConflict, fmt.Errorf("project %q already exists", newName)}
		}
	}
	dir := p.store.Root
	if newDir != "" {
		dir = newDir
	}
	repoint := filepath.Clean(dir) != filepath.Clean(p.store.Root)
	if newName == name && !repoint {
		return nil // nothing to change
	}

	prev := append([]string(nil), s.order...)
	if repoint {
		p.hub.broadcast() // wake browsers on the same URL to refetch the new tree
	}
	if p.cancelWatch != nil {
		p.cancelWatch()
		p.cancelWatch = nil
	}
	delete(s.projects, name)
	np := &project{name: newName, store: store.Open(dir), hub: newHub()}
	s.projects[newName] = np
	s.startWatchLocked(np)
	for i, n := range s.order {
		if n == name {
			s.order[i] = newName
			break
		}
	}

	if err := s.persistLocked(); err != nil {
		// A disk error leaves the new mount live but the file un-updated; restore the
		// order so the picker stays self-consistent, and let the next mutation (or a
		// restart, which reloads the file) reconcile.
		s.order = prev
		return &httpError{http.StatusInternalServerError, fmt.Errorf("persist registry: %w", err)}
	}
	return nil
}

// persistLocked writes the current registry back to projects.yaml. The caller
// holds regmu. It is a no-op when no registry file is configured (tests), so the
// in-memory registry still mutates without needing a temp file on disk.
func (s *Server) persistLocked() error {
	if s.regFile == "" {
		return nil
	}
	list := make([]registry.Project, 0, len(s.order))
	for _, name := range s.order {
		list = append(list, registry.Project{Name: name, Dir: s.projects[name].store.Root})
	}
	return registry.Save(s.regFile, list)
}

// isThingTree reports whether dir is an initialized thing tree — marked by a
// config.yaml (written by `thing init`). Requiring it keeps mounts to
// already-initialized directories rather than an arbitrary or empty path.
func isThingTree(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, config.FileName))
	return err == nil && !fi.IsDir()
}

// mountLocked adds a mount for name over dir and starts its watcher, returning
// the new project. The caller holds regmu and has already validated name and dir;
// it does not persist.
func (s *Server) mountLocked(name, dir string) *project {
	p := &project{name: name, store: store.Open(dir), hub: newHub()}
	s.projects[name] = p
	s.order = append(s.order, name)
	s.startWatchLocked(p)
	return p
}

// unmountLocked stops name's watcher and drops its mount from the registry
// (projects and order). The caller holds regmu. It leaves the data directory on
// disk and does not persist; an unknown name is a no-op.
func (s *Server) unmountLocked(name string) {
	p, ok := s.projects[name]
	if !ok {
		return
	}
	if p.cancelWatch != nil {
		p.cancelWatch()
		p.cancelWatch = nil
	}
	delete(s.projects, name)
	for i, n := range s.order {
		if n == name {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// startWatchLocked launches p's filesystem watcher under a child of the server's
// watch context and records its cancel. The caller holds regmu. It is a no-op
// before StartWatch has run (watchCtx nil) or if p is already watching, so
// Register is safe whether it runs before or after watching begins.
func (s *Server) startWatchLocked(p *project) {
	if s.watchCtx == nil || p.cancelWatch != nil {
		return
	}
	ctx, cancel := context.WithCancel(s.watchCtx)
	p.cancelWatch = cancel
	go p.watch(ctx, s.watchInterval)
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

// registerReq is the PUT /api/projects/{project} body. The name is the {project}
// path segment (the resource URI); the body carries only the data directory to
// mount, which must already be an initialized thing tree.
type registerReq struct {
	Dir string `json:"dir"`
}

// handleRegister mounts the project named in the path over the body's dir. It is
// a PUT upsert: a fresh mount returns 201, a repeat of an existing identical
// mount returns 200, and it surfaces the error's status otherwise (400 bad
// name/dir, 409 name bound to a different dir).
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	p, created, err := s.Register(r.PathValue("project"), strings.TrimSpace(req.Dir))
	if err != nil {
		s.failErr(w, err)
		return
	}
	dir := p.store.Root
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	code := http.StatusOK
	if created {
		code = http.StatusCreated
	}
	writeJSON(w, code, map[string]string{"name": p.name, "dir": dir})
}

// projectPatchReq is the PATCH /api/projects/{project} body. It carries one of
// two operations: a reorder (before/after — place this project relative to an
// anchor; the front is before the current first, the end after the current last)
// or an edit (name/dir — rename and/or re-point). name and dir are pointers so an
// omitted field ("keep it") is distinct from an explicit empty one (rejected).
type projectPatchReq struct {
	Before string  `json:"before"`
	After  string  `json:"after"`
	Name   *string `json:"name"`
	Dir    *string `json:"dir"`
}

// handlePatchProject dispatches the two PATCH operations on a project: a reorder
// (before/after) or an edit (name/dir). Exactly one kind must be present — both
// together, or neither, is a 400. It 404s an unknown project and surfaces the
// operation's own status otherwise (400 bad anchor/name/dir, 409 name taken).
func (s *Server) handlePatchProject(w http.ResponseWriter, r *http.Request) {
	var req projectPatchReq
	if !decode(w, r, &req) {
		return
	}
	name := r.PathValue("project")
	isMove := req.Before != "" || req.After != ""
	isEdit := req.Name != nil || req.Dir != nil

	switch {
	case isMove && isEdit:
		s.fail(w, http.StatusBadRequest, fmt.Errorf("specify a move (before/after) or an edit (name/dir), not both"))
	case isMove:
		if err := s.Move(name, strings.TrimSpace(req.Before), strings.TrimSpace(req.After)); err != nil {
			s.failErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case isEdit:
		newName := name
		if req.Name != nil {
			if newName = strings.TrimSpace(*req.Name); newName == "" {
				s.fail(w, http.StatusBadRequest, fmt.Errorf("name cannot be empty"))
				return
			}
		}
		newDir := ""
		if req.Dir != nil {
			if newDir = strings.TrimSpace(*req.Dir); newDir == "" {
				s.fail(w, http.StatusBadRequest, fmt.Errorf("dir cannot be empty"))
				return
			}
		}
		if err := s.Edit(name, newName, newDir); err != nil {
			s.failErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		s.fail(w, http.StatusBadRequest, fmt.Errorf("empty patch: specify before/after or name/dir"))
	}
}

// handleReload re-reads projects.yaml and reconciles the registry to it (see
// Reload), returning a summary of what changed. It is the picker's refresh action
// — the one route that resyncs the whole registry from disk.
func (s *Server) handleReload(w http.ResponseWriter, _ *http.Request) {
	res, err := s.Reload()
	if err != nil {
		s.failErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleUnregister removes the project named in the path (404 when unknown). It
// unregisters only; the data directory is left on disk.
func (s *Server) handleUnregister(w http.ResponseWriter, r *http.Request) {
	if err := s.Unregister(r.PathValue("project")); err != nil {
		s.failErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// failErr writes err with its carried HTTP status when it is an *httpError, else
// 500. It keeps Register/Unregister handlers from re-deriving status codes.
func (s *Server) failErr(w http.ResponseWriter, err error) {
	if he, ok := errors.AsType[*httpError](err); ok {
		s.fail(w, he.code, he)
		return
	}
	s.fail(w, http.StatusInternalServerError, err)
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

// configRes is the display config the web UI reads: the title (from config.yaml
// in the served data directory), the data directory path shown as a label, and
// the filter state the board starts from.
type configRes struct {
	Title  string     `json:"title"`
	Dir    string     `json:"dir"`
	Filter *filterRes `json:"filter,omitempty"`
}

// filterRes is the resolved default filter. Empty facets are omitted, so on the
// wire — as in config.yaml — an absent key means "this facet is not filtered".
type filterRes struct {
	Statuses   []model.Status   `json:"statuses,omitempty"`
	Priorities []model.Priority `json:"priorities,omitempty"`
	Category   string           `json:"category,omitempty"`
	Tag        string           `json:"tag,omitempty"`
	Query      string           `json:"query,omitempty"`
}

// newFilterRes converts a resolved filter to its wire form, returning nil when
// nothing is filtered — including the case where every configured facet resolved
// to empty, which is indistinguishable from no configuration at all.
func newFilterRes(f *config.Filter) *filterRes {
	if f == nil {
		return nil
	}
	res := &filterRes{
		Statuses:   f.Statuses,
		Priorities: f.Priorities,
		Category:   f.Category,
		Tag:        f.Tag,
		Query:      f.Query,
	}
	if len(res.Statuses) == 0 && len(res.Priorities) == 0 &&
		res.Category == "" && res.Tag == "" && res.Query == "" {
		return nil
	}
	return res
}

// globalConfig loads the global config.yaml, whose filter block supplies defaults
// for every project. THING_CONFIG_DIR overrides where it lives, matching the CLI's
// env knob. An unresolvable home directory is not fatal: it just means no global
// defaults.
func globalConfig() (*config.Config, error) {
	dir := os.Getenv("THING_CONFIG_DIR")
	if dir == "" {
		var err error
		if dir, err = store.GlobalConfigDir(); err != nil {
			return &config.Config{}, nil
		}
	}
	return config.Load(dir)
}

// handleConfig serves the display config the web UI reads. The title comes from
// config.yaml in the served data directory — the project-local .thing/ holds it
// alongside the nodes — and a missing or title-less config yields the default
// "thing", so the endpoint always returns a usable title. The filter defaults are
// layered: the global config.yaml applies to every project, and the project's own
// filter block overrides it key by key.
func (s *Server) handleConfig(p *project, w http.ResponseWriter, _ *http.Request) {
	p.mu.RLock()
	root := p.store.Root
	cfg, err := config.Load(root)
	p.mu.RUnlock()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	global, err := globalConfig()
	if err != nil {
		// A broken global config.yaml fails this endpoint for every project, not
		// just this one, and s.fail itself is silent — log so the failure is
		// discoverable in the thingd terminal rather than only as an opaque 500.
		if s.logger != nil {
			s.logger.Printf("load global config: %v", err)
		}
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
	writeJSON(w, http.StatusOK, configRes{
		Title:  title,
		Dir:    dir,
		Filter: newFilterRes(config.ResolveFilter(global, cfg)),
	})
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
	Archive    *bool    `json:"archive"`    // true archives the node into _archives/
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
	for _, present := range []bool{frontmatter, req.AddLink != nil, req.RemoveLink != nil, req.Move != nil, req.Archive != nil} {
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

	// Archive moves the node out of the live tree; report its archive ref.
	if req.Archive != nil {
		if !*req.Archive {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("archive must be true"))
			return
		}
		ref, err := p.store.Archive(e, s.nowStamp())
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ref": ref})
		return
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

// handleArchiveList serves the archived entries: each entry's archive ref, the
// ref it was archived from, its title, type, priority, own status, and the
// RFC3339 time it was archived. It reads under the project's read lock, like the
// tree. Archived subtrees are not loaded past their top node, so the row carries
// the node's own status rather than a status rolled up from children it never
// loaded (which would collapse every rollup node to "todo").
func (s *Server) handleArchiveList(p *project, w http.ResponseWriter, _ *http.Request) {
	p.mu.RLock()
	entries, err := p.store.ArchiveList()
	p.mu.RUnlock()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	type item struct {
		Ref        string         `json:"ref"`
		From       string         `json:"from"`
		Title      string         `json:"title"`
		Type       model.NodeType `json:"type"`
		Priority   model.Priority `json:"priority,omitempty"`
		Status     model.Status   `json:"status,omitempty"`
		ArchivedAt string         `json:"archivedAt,omitempty"`
	}
	out := make([]item, 0, len(entries))
	for _, e := range entries {
		out = append(out, item{
			Ref:        e.Ref,
			From:       e.Node.ArchivedRef,
			Title:      e.Node.Title,
			Type:       e.Node.Type,
			Priority:   e.Node.Priority,
			Status:     e.Node.Status,
			ArchivedAt: e.Node.ArchivedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleArchive serves one archived entry's detail — where it came from, its own
// status, and its body — the web equivalent of `show _archives/<name>`. Children
// are not included (they travel with the subtree and are not loaded); a missing
// entry is a 404.
func (s *Server) handleArchive(p *project, w http.ResponseWriter, r *http.Request) {
	ref := store.ArchiveDir + "/" + r.PathValue("name")
	p.mu.RLock()
	ae, err := p.store.ArchiveLocate(ref)
	p.mu.RUnlock()
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if ae == nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("no such archived node %q", ref))
		return
	}
	n := ae.Node
	writeJSON(w, http.StatusOK, struct {
		Ref        string         `json:"ref"`
		From       string         `json:"from"`
		Type       model.NodeType `json:"type"`
		Title      string         `json:"title"`
		Status     model.Status   `json:"status,omitempty"`
		Priority   model.Priority `json:"priority,omitempty"`
		Category   string         `json:"category,omitempty"`
		Tags       []string       `json:"tags,omitempty"`
		Links      []model.Link   `json:"links,omitempty"`
		Body       string         `json:"body,omitempty"`
		ArchivedAt string         `json:"archivedAt,omitempty"`
	}{
		Ref:        ae.Ref,
		From:       n.ArchivedRef,
		Type:       n.Type,
		Title:      n.Title,
		Status:     n.Status,
		Priority:   n.Priority,
		Category:   n.Category,
		Tags:       n.Tags,
		Links:      n.Links,
		Body:       n.Body,
		ArchivedAt: n.ArchivedAt,
	})
}

// unarchiveReq is the PATCH /archives/{name} body: an optional destination that
// overrides the recorded source (the ref it was archived from).
type unarchiveReq struct {
	To string `json:"to"`
}

// handleUnarchive restores the archived entry named in the path. A missing entry
// is a 404; a restore that collides or whose parent is gone is a 400 (retry with
// "to"). On success it returns the restored ref.
func (s *Server) handleUnarchive(p *project, w http.ResponseWriter, r *http.Request) {
	var req unarchiveReq
	if !decode(w, r, &req) {
		return
	}
	ref := store.ArchiveDir + "/" + r.PathValue("name")
	p.mu.Lock()
	defer p.mu.Unlock()
	ae, err := p.store.ArchiveLocate(ref)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	if ae == nil {
		s.fail(w, http.StatusNotFound, fmt.Errorf("no such archived node %q", ref))
		return
	}
	newRef, err := p.store.Unarchive(ae, strings.TrimSpace(req.To), s.now())
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ref": newRef})
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
