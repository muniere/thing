package cli

import (
	"strings"
	"testing"
)

func TestServerHelpListsSubcommands(t *testing.T) {
	_, out, _ := runCLI(t, "server", "--help")
	for _, sub := range []string{"start", "stop", "restart", "status", "logs"} {
		if !strings.Contains(out, sub) {
			t.Errorf("server --help missing %q\n%s", sub, out)
		}
	}
}

func TestServerStatusStopped(t *testing.T) {
	// An empty state dir means no daemon: status prints "stopped" on stdout and
	// exits 1 (systemctl-style).
	t.Setenv("THING_DATA_DIR", t.TempDir())
	code, out, errb := runCLI(t, "server", "status")
	if code != 1 {
		t.Errorf("stopped status: code=%d, want 1", code)
	}
	if strings.TrimSpace(out) != "stopped" {
		t.Errorf("stopped status stdout = %q, want \"stopped\"", out)
	}
	if errb != "" {
		t.Errorf("stopped status should not write stderr, got %q", errb)
	}
}

func TestServerStopWhenNotRunning(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	code, out, _ := runCLI(t, "server", "stop")
	if code != 0 {
		t.Errorf("stop when down: code=%d, want 0", code)
	}
	if strings.TrimSpace(out) != "not running" {
		t.Errorf("stop when down stdout = %q, want \"not running\"", out)
	}
}

func TestServerLogsNoLog(t *testing.T) {
	t.Setenv("THING_DATA_DIR", t.TempDir())
	code, out, _ := runCLI(t, "server", "logs")
	if code != 0 {
		t.Errorf("logs with no file: code=%d, want 0", code)
	}
	if strings.TrimSpace(out) != "no log yet" {
		t.Errorf("logs with no file stdout = %q, want \"no log yet\"", out)
	}
}
