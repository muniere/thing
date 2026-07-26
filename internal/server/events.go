package server

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// heartbeatInterval is how often an idle SSE stream sends a comment frame, to
// keep the connection alive through proxies and surface a dead peer as a write
// error even when no reload fires.
const heartbeatInterval = 30 * time.Second

// hub is a minimal SSE fan-out: subscribers register a channel and receive a
// signal whenever the tree changes. It carries no payload — clients refetch
// /api/tree on any event.
type hub struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func newHub() *hub {
	return &hub{subs: make(map[chan struct{}]struct{})}
}

func (h *hub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// broadcast wakes every subscriber. A full buffer means a signal is already
// pending, so the send is dropped — coalescing bursts into one refetch.
func (h *hub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// handleEvents streams reload signals over Server-Sent Events until the client
// disconnects.
func (s *Server) handleEvents(p *project, w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.fail(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := p.hub.subscribe()
	defer p.hub.unsubscribe(ch)

	// send writes one frame and flushes; a write error means the client is gone,
	// so the caller returns and the deferred unsubscribe cleans up.
	send := func(frame string) bool {
		if _, err := io.WriteString(w, frame); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Greet immediately so the client knows the stream is live. The data is this
	// process's bootID: a client that reconnects to a different id knows the
	// server was replaced (e.g. air rebuilt the binary in dev) and does a full
	// reload rather than a plain refetch.
	if !send("event: hello\ndata: " + s.bootID + "\n\n") {
		return
	}

	ping := time.NewTicker(heartbeatInterval)
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			if !send(": ping\n\n") {
				return
			}
		case <-ch:
			if !send("event: reload\ndata: tree\n\n") {
				return
			}
		}
	}
}

// StartWatch launches one filesystem poller per registered project and blocks
// until ctx is cancelled. Each poller broadcasts a reload to its own project's
// hub whenever that project's data directory changes, so edits made outside the
// web (CLI, editor) refresh only that project's open browsers. It also records
// ctx and interval so a project registered later (via Register) spins up its own
// watcher under the same lifetime.
func (s *Server) StartWatch(ctx context.Context, interval time.Duration) {
	s.regmu.Lock()
	s.watchCtx = ctx
	s.watchInterval = interval
	for _, p := range s.projects {
		s.startWatchLocked(p)
	}
	s.regmu.Unlock()
	<-ctx.Done()
}

// watch polls the project's data directory and broadcasts a reload to its hub
// whenever the directory's fingerprint changes. It returns when ctx is cancelled.
func (p *project) watch(ctx context.Context, interval time.Duration) {
	last := fingerprint(p.store.Root)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if fp := fingerprint(p.store.Root); fp != last {
				last = fp
				p.hub.broadcast()
			}
		}
	}
}

// fingerprint hashes each file's path, size, and mtime — cheap, and enough to
// notice a create, delete, rename, or (via mtime) content write. Per-entry walk
// errors skip that file rather than aborting the walk, so a briefly unreadable
// file drops out of the hash and triggers one extra reload — harmless, since a
// reload just refetches the whole tree.
func fingerprint(root string) uint64 {
	type ent struct {
		path string
		size int64
		mod  int64
	}
	var ents []ent
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		ents = append(ents, ent{path, info.Size(), info.ModTime().UnixNano()})
		return nil
	})
	sort.Slice(ents, func(i, j int) bool { return ents[i].path < ents[j].path })
	h := fnv.New64a()
	for _, e := range ents {
		_, _ = h.Write([]byte(e.path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(strconv.FormatInt(e.size, 10)))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(strconv.FormatInt(e.mod, 10)))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}
