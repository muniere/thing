package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFingerprintChangesOnEdit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := fingerprint(root)
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fingerprint(root) == before {
		t.Fatal("fingerprint unchanged after adding a file")
	}
}

func TestEventsStreamsReloadOnMutation(t *testing.T) {
	s := newServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	br := bufio.NewReader(resp.Body)
	// First frame is the hello greeting.
	if line := readEvent(t, br); !strings.Contains(line, "hello") {
		t.Fatalf("first event = %q, want hello", line)
	}

	// A mutation over the API must push a reload frame.
	go func() {
		time.Sleep(50 * time.Millisecond)
		req, _ := http.NewRequest("PATCH", ts.URL+"/api/nodes/alpha/one/do-it", strings.NewReader(`{"status":"done"}`))
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req)
	}()

	if line := readEvent(t, br); !strings.Contains(line, "reload") {
		t.Fatalf("event after mutation = %q, want reload", line)
	}
}

// readEvent reads one SSE frame (up to a blank line) and returns it joined.
func readEvent(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	var b strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("reading event: %v", err)
		}
		if line == "\n" || line == "\r\n" {
			return b.String()
		}
		b.WriteString(line)
	}
}

func TestHubCoalesceAndFanout(t *testing.T) {
	h := newHub()
	a, b := h.subscribe(), h.subscribe()
	// Three bursts coalesce (buffer cap 1); both subscribers still get the signal.
	h.broadcast()
	h.broadcast()
	h.broadcast()
	for i, ch := range []chan struct{}{a, b} {
		select {
		case <-ch:
		default:
			t.Fatalf("subscriber %d missed the broadcast", i)
		}
		select {
		case <-ch:
			t.Fatalf("subscriber %d: bursts should coalesce to one signal", i)
		default:
		}
	}
}

func TestHubUnsubscribeCleanup(t *testing.T) {
	h := newHub()
	ch := h.subscribe()
	if len(h.subs) != 1 {
		t.Fatalf("subs = %d, want 1", len(h.subs))
	}
	h.unsubscribe(ch)
	if len(h.subs) != 0 {
		t.Fatalf("subs = %d after unsubscribe, want 0 (leak)", len(h.subs))
	}
}

func TestBroadcastGating(t *testing.T) {
	s := newServer(t)
	ch := s.hub.subscribe()
	// A read and a failed mutation must not broadcast a reload.
	do(t, s, "GET", "/api/tree", "")
	do(t, s, "PATCH", "/api/nodes/nope", `{"status":"done"}`)
	select {
	case <-ch:
		t.Fatal("a GET or a failed mutation broadcast a reload")
	default:
	}
	// A successful mutation does.
	do(t, s, "PATCH", "/api/nodes/alpha/one/do-it", `{"status":"done"}`)
	select {
	case <-ch:
	default:
		t.Fatal("a successful mutation did not broadcast")
	}
}

func TestStartWatchDetectsDiskEdit(t *testing.T) {
	s := newServer(t)
	ts := httptest.NewServer(s)
	defer ts.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.StartWatch(ctx, 20*time.Millisecond)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", ts.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)
	if line := readEvent(t, br); !strings.Contains(line, "hello") {
		t.Fatalf("first event = %q, want hello", line)
	}

	// An out-of-band edit (write a file directly, bypassing the API) reloads via
	// the fingerprint poller — the path StartWatch exists for.
	if err := os.WriteFile(filepath.Join(s.store.Root, "zzz.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if line := readEvent(t, br); !strings.Contains(line, "reload") {
		t.Fatalf("event after disk edit = %q, want reload", line)
	}
}

func TestFingerprintStable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, second := fingerprint(root), fingerprint(root)
	if first != second {
		t.Fatal("fingerprint of an unchanged tree differs between calls")
	}
}
