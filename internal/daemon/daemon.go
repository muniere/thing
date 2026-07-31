package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/muniere/thing/internal/server"
)

// ErrNotRunning is returned by Stop when no daemon is running.
var ErrNotRunning = errors.New("thingd is not running")

// ErrAlreadyRunning is returned by Start when a daemon is already running.
var ErrAlreadyRunning = errors.New("thingd is already running")

// DefaultPort is the port the daemon binds by default. Unlike a foreground
// thingd, the daemon pins this port (never hops) so its URL stays stable.
const DefaultPort = server.DefaultPort

// binEnv overrides thingd binary resolution when set.
const binEnv = "THINGD_BIN"

// startupGrace is how long Start waits for a freshly launched thingd to prove it
// stays up. A common early exit is the port already being taken.
const startupGrace = 400 * time.Millisecond

// ResolveBinary locates the thingd executable, in order:
//
//	$THINGD_BIN                         -> used verbatim when set
//	sibling of the running thing binary -> ./thingd next to this executable
//	$PATH                               -> exec.LookPath("thingd")
//
// The sibling lookup makes `go install ./cmd/...` and a local `bin/` build both
// work without configuration, since thing and thingd land together.
func ResolveBinary() (string, error) {
	if v := os.Getenv(binEnv); v != "" {
		return v, nil
	}
	if self, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(self), "thingd")
		if isExecutable(cand) {
			return cand, nil
		}
	}
	if path, err := exec.LookPath("thingd"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("thingd binary not found: set %s, or install thingd on PATH or next to thing", binEnv)
}

// isExecutable reports whether path is a regular file with any execute bit set.
func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode()&0o111 != 0
}

// alive reports whether a process with the given pid exists. Signal 0 performs
// error checking without delivering a signal: nil or EPERM means the process is
// there (EPERM = it exists but is owned by another user), ESRCH means it is gone.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Status reads the recorded state and reports whether the daemon is running. A
// state file whose process has died is stale: it is removed and reported as
// stopped, so a crashed daemon doesn't wedge future starts.
func Status() (st State, running bool, err error) {
	path, err := StatePath()
	if err != nil {
		return State{}, false, err
	}
	st, ok, err := readState(path)
	if err != nil || !ok {
		return State{}, false, err
	}
	if !alive(st.PID) {
		if err := removeState(path); err != nil {
			return State{}, false, err
		}
		return State{}, false, nil
	}
	return st, true, nil
}

// StartOptions configures a daemon launch.
type StartOptions struct {
	// Port is the port thingd binds. Zero means DefaultPort.
	Port int
}

// Start launches thingd detached on the configured port and records its runtime
// state. It returns ErrAlreadyRunning if a daemon is already up. If thingd exits
// within the startup grace period (typically because the port is taken), Start
// reports the failure and leaves no state behind; the reason is in the log.
func Start(opts StartOptions) (State, error) {
	if _, running, err := Status(); err != nil {
		return State{}, err
	} else if running {
		return State{}, ErrAlreadyRunning
	}

	port := opts.Port
	if port == 0 {
		port = DefaultPort
	}
	bin, err := ResolveBinary()
	if err != nil {
		return State{}, err
	}
	logPath, err := LogPath()
	if err != nil {
		return State{}, err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return State{}, err
	}
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return State{}, err
	}
	defer logf.Close() // the child keeps its own inherited fd after Start.

	cmd := exec.Command(bin, "--port", strconv.Itoa(port))
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach into its own session.
	if err := cmd.Start(); err != nil {
		return State{}, fmt.Errorf("launch thingd: %w", err)
	}

	// Watch for a fast crash (e.g. port in use). Reaping via Wait in a goroutine
	// also avoids a lingering zombie if it does exit early.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case werr := <-exited:
		return State{}, fmt.Errorf("thingd exited immediately (%v); see %s", werr, logPath)
	case <-time.After(startupGrace):
	}

	st := State{
		PID:       cmd.Process.Pid,
		Port:      port,
		URL:       fmt.Sprintf("http://localhost:%d", port),
		StartedAt: time.Now().Format(time.RFC3339),
	}
	statePath, err := StatePath()
	if err != nil {
		return State{}, err
	}
	if err := writeState(statePath, st); err != nil {
		return State{}, err
	}
	return st, nil
}

// stopGrace bounds how long Stop waits for a graceful (SIGTERM) exit before it
// escalates to SIGKILL.
const stopGrace = 3 * time.Second

// Stop terminates a running daemon: SIGTERM, then SIGKILL if it outlasts the
// grace period, and finally removes the state file. It returns ErrNotRunning if
// no daemon is up.
func Stop() error {
	st, running, err := Status()
	if err != nil {
		return err
	}
	if !running {
		return ErrNotRunning
	}
	if err := syscall.Kill(st.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal thingd: %w", err)
	}
	deadline := time.Now().Add(stopGrace)
	for alive(st.PID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if alive(st.PID) {
		_ = syscall.Kill(st.PID, syscall.SIGKILL)
	}
	path, err := StatePath()
	if err != nil {
		return err
	}
	return removeState(path)
}
