package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeThingd drops an executable script named "thingd" in dir that runs the
// given shell body (ignoring the --port args thing passes), and returns its path.
func writeFakeThingd(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "thingd")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake thingd: %v", err)
	}
	return path
}

func TestResolveBinaryEnvWins(t *testing.T) {
	t.Setenv(binEnv, "/custom/thingd")
	got, err := ResolveBinary()
	if err != nil {
		t.Fatalf("ResolveBinary: %v", err)
	}
	if got != "/custom/thingd" {
		t.Errorf("got %q, want /custom/thingd", got)
	}
}

func TestResolveBinaryFromPath(t *testing.T) {
	// No env override and no sibling: falls through to PATH.
	t.Setenv(binEnv, "")
	dir := t.TempDir()
	want := writeFakeThingd(t, dir, "exit 0")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := ResolveBinary()
	if err != nil {
		t.Fatalf("ResolveBinary: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveBinaryNotFound(t *testing.T) {
	t.Setenv(binEnv, "")
	t.Setenv("PATH", t.TempDir()) // empty dir on PATH, no sibling thingd
	if _, err := ResolveBinary(); err == nil {
		t.Error("expected an error when thingd is nowhere")
	}
}

func TestAlive(t *testing.T) {
	if !alive(os.Getpid()) {
		t.Error("current process should be alive")
	}
	if alive(-1) {
		t.Error("pid -1 should not be alive")
	}
	// A process we run to completion is dead; its pid should report not alive.
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run true: %v", err)
	}
	if alive(cmd.Process.Pid) {
		t.Errorf("pid %d should be dead after Wait", cmd.Process.Pid)
	}
}

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	path, err := StatePath()
	if err != nil {
		t.Fatalf("StatePath: %v", err)
	}
	want := State{PID: 4242, Port: 4319, URL: "http://localhost:4319", StartedAt: "2026-07-31T00:00:00Z"}
	if err := writeState(path, want); err != nil {
		t.Fatalf("writeState: %v", err)
	}
	// Atomic write leaves no temp files behind.
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %q", e.Name())
		}
	}
	got, ok, err := readState(path)
	if err != nil || !ok {
		t.Fatalf("readState: ok=%v err=%v", ok, err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestStatusStaleIsCleaned(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	path, _ := StatePath()
	// A dead pid recorded on disk: Status reports stopped and removes the file.
	cmd := exec.Command("true")
	_ = cmd.Run()
	if err := writeState(path, State{PID: cmd.Process.Pid, Port: 4319}); err != nil {
		t.Fatalf("writeState: %v", err)
	}
	_, running, err := Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if running {
		t.Error("stale state should report not running")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("stale state file should be removed")
	}
}

func TestStartStopLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("THING_DATA_DIR", dir)
	t.Setenv(binEnv, writeFakeThingd(t, dir, "exec sleep 30"))

	st, err := Start(StartOptions{Port: 4319})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st.PID <= 0 || st.Port != 4319 || st.URL != "http://localhost:4319" {
		t.Errorf("unexpected state %+v", st)
	}
	if !alive(st.PID) {
		t.Fatalf("pid %d should be running after Start", st.PID)
	}

	// A second Start refuses while one is up.
	if _, err := Start(StartOptions{Port: 4319}); err != ErrAlreadyRunning {
		t.Errorf("second Start: got %v, want ErrAlreadyRunning", err)
	}

	if err := Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if alive(st.PID) {
		t.Errorf("pid %d should be stopped", st.PID)
	}
	path, _ := StatePath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("state file should be gone after Stop")
	}

	// Stop with nothing running is ErrNotRunning.
	if err := Stop(); err != ErrNotRunning {
		t.Errorf("Stop when down: got %v, want ErrNotRunning", err)
	}
}

func TestStartFailsWhenThingdExitsImmediately(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("THING_DATA_DIR", dir)
	t.Setenv(binEnv, writeFakeThingd(t, dir, "exit 3"))

	if _, err := Start(StartOptions{Port: 4319}); err == nil {
		t.Fatal("expected Start to fail when thingd exits immediately")
	}
	// No state should be recorded for a failed start.
	path, _ := StatePath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("no state file should exist after a failed start")
	}
}

func TestReadLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok, err := ReadLastLines(path, 2)
	if err != nil || !ok {
		t.Fatalf("ReadLastLines: ok=%v err=%v", ok, err)
	}
	if len(got) != 2 || got[0] != "d" || got[1] != "e" {
		t.Errorf("got %v, want [d e]", got)
	}
	// n<=0 returns everything.
	all, _, _ := ReadLastLines(path, 0)
	if len(all) != 5 {
		t.Errorf("got %d lines, want 5", len(all))
	}
	// Missing file: ok=false, no error.
	if _, ok, err := ReadLastLines(filepath.Join(dir, "absent.txt"), 5); ok || err != nil {
		t.Errorf("missing file: ok=%v err=%v, want false,nil", ok, err)
	}
}

func TestFollow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var sink strings.Builder
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- Follow(path, &sink, stop) }()

	// Append after Follow has started from the end; it should stream the new line.
	time.Sleep(250 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("fresh\n")
	f.Close()
	time.Sleep(400 * time.Millisecond)
	close(stop)
	if err := <-done; err != nil {
		t.Fatalf("Follow: %v", err)
	}
	if got := sink.String(); !strings.Contains(got, "fresh") || strings.Contains(got, "old") {
		t.Errorf("followed output = %q, want it to contain 'fresh' and not 'old'", got)
	}
}
