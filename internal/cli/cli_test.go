package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI executes the root command with the given args and returns exit code,
// stdout, and stderr — mirroring how the binary reports errors.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	root := NewRootCmd("0.0.1-test")
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	code := 0
	if err := root.Execute(); err != nil {
		fmt.Fprintln(&errb, "thing:", err)
		code = 1
	}
	return code, out.String(), errb.String()
}

func TestWalkingSkeleton(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}

	// init
	if code, out, errb := runCLI(t, append([]string{"init"}, d...)...); code != 0 {
		t.Fatalf("init: code=%d err=%s", code, errb)
	} else if !strings.Contains(out, "initialized") {
		t.Fatalf("init out: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}

	// epic add
	code, out, errb := runCLI(t, append([]string{"epic", "add", "Web release"}, d...)...)
	if code != 0 {
		t.Fatalf("epic add: code=%d err=%s", code, errb)
	}
	epicSlug := strings.TrimSpace(out)
	if epicSlug != "web-release" {
		t.Fatalf("epic slug = %q, want web-release", epicSlug)
	}

	// issue add under epic
	_, out, errb = runCLI(t, append([]string{"issue", "add", "Monitor rollout", "--epic", epicSlug}, d...)...)
	issueSlug := strings.TrimSpace(out)
	if issueSlug != "monitor-rollout" {
		t.Fatalf("issue slug = %q err=%s", issueSlug, errb)
	}

	// task add requires --issue
	if code, _, _ := runCLI(t, append([]string{"task", "add", "no parent"}, d...)...); code == 0 {
		t.Fatal("task add without --issue should fail")
	}

	// task add under issue
	_, out, _ = runCLI(t, append([]string{"task", "add", "Confirm routing", "--issue", issueSlug}, d...)...)
	if strings.TrimSpace(out) != "confirm-routing" {
		t.Fatalf("task slug = %q", out)
	}

	// orphan issue (no --epic)
	_, out, _ = runCLI(t, append([]string{"issue", "add", "Loose end"}, d...)...)
	if strings.TrimSpace(out) != "loose-end" {
		t.Fatalf("orphan slug = %q", out)
	}

	// tree shows everything
	code, out, errb = runCLI(t, append([]string{"tree"}, d...)...)
	if code != 0 {
		t.Fatalf("tree: code=%d err=%s", code, errb)
	}
	for _, want := range []string{"web-release", "monitor-rollout", "confirm-routing", "loose-end"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree missing %q\n%s", want, out)
		}
	}
}

func TestSlugUniqueness(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	_, out1, _ := runCLI(t, append([]string{"epic", "add", "Same Name"}, d...)...)
	_, out2, _ := runCLI(t, append([]string{"epic", "add", "Same Name"}, d...)...)
	if strings.TrimSpace(out1) != "same-name" || strings.TrimSpace(out2) != "same-name-2" {
		t.Errorf("uniqueness: %q then %q", out1, out2)
	}
}

func TestVersionAndUsage(t *testing.T) {
	if code, out, _ := runCLI(t, "--version"); code != 0 || strings.TrimSpace(out) == "" {
		t.Errorf("version: code=%d out=%q", code, out)
	}
	if code, out, _ := runCLI(t); code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("usage: code=%d out=%q", code, out)
	}
	if code, _, _ := runCLI(t, "bogus"); code == 0 {
		t.Error("unknown command should be non-zero")
	}
}
