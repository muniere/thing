package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintStartup(t *testing.T) {
	var buf bytes.Buffer
	PrintStartup(&buf, "1.2.3", "http://localhost:4319", "/tmp/.thing")
	out := buf.String()
	for _, want := range []string{"thing", "1.2.3", "http://localhost:4319", "/tmp/.thing", "Local:", "Dir:", "Ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q; got:\n%s", want, out)
		}
	}
	// Writing to a non-terminal (a buffer) emits no ANSI escapes.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-terminal output should carry no ANSI codes; got:\n%q", out)
	}
}
