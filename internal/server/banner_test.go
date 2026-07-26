package server

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintStartup(t *testing.T) {
	var buf bytes.Buffer
	PrintStartup(&buf, "1.2.3", "http://localhost:4319", []ProjectInfo{
		{Name: "work", Dir: "/tmp/work/.thing"},
		{Name: "home", Dir: "/tmp/home/.thing"},
	})
	out := buf.String()
	// The banner shows the base URL, each project's name, its per-project URL, and
	// its data directory.
	for _, want := range []string{
		"thing", "1.2.3", "http://localhost:4319", "Ready",
		"work", "http://localhost:4319/work", "/tmp/work/.thing",
		"home", "http://localhost:4319/home", "/tmp/home/.thing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q; got:\n%s", want, out)
		}
	}
	// Writing to a non-terminal (a buffer) emits no ANSI escapes.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-terminal output should carry no ANSI codes; got:\n%q", out)
	}
}

func TestPrintStartupNoProjects(t *testing.T) {
	var buf bytes.Buffer
	PrintStartup(&buf, "1.2.3", "http://localhost:4319", nil)
	out := buf.String()
	// With no projects registered, the banner still renders and hints how to add one.
	for _, want := range []string{"thing", "http://localhost:4319", "No projects"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty banner missing %q; got:\n%s", want, out)
		}
	}
}
