package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestListAndShow(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"epic", "add", "Web"}, d...)...)
	runCLI(t, append([]string{"issue", "add", "Roll", "--epic", "web"}, d...)...)
	runCLI(t, append([]string{"task", "add", "Do", "--issue", "roll"}, d...)...)

	// list is scoped per resource.
	if _, out, _ := runCLI(t, append([]string{"epic", "list"}, d...)...); !strings.Contains(out, "web") {
		t.Errorf("epic list: %q", out)
	}
	if _, out, _ := runCLI(t, append([]string{"task", "list", "--issue", "roll"}, d...)...); !strings.Contains(out, "do") {
		t.Errorf("task list --issue: %q", out)
	}

	// show reflects a node; the resource acts as a type guard.
	if _, out, _ := runCLI(t, append([]string{"task", "show", "do"}, d...)...); !strings.Contains(out, "Do") {
		t.Errorf("show: %q", out)
	}
	if code, _, _ := runCLI(t, append([]string{"epic", "show", "do"}, d...)...); code == 0 {
		t.Error("epic show on a task slug should fail")
	}
}

func TestSplitTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}}, // surrounding whitespace trimmed
		{"a,,b", []string{"a", "b"}},          // blank fields dropped
	}
	for _, c := range cases {
		got := splitTrim(c.in, ",")
		if len(got) != len(c.want) {
			t.Errorf("splitTrim(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitTrim(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestTagsFlag(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)

	// --tags is comma-separated; whitespace around each tag is trimmed on the
	// way into the stored node.
	_, out, _ := runCLI(t, append([]string{"epic", "add", "Tagged", "--tags", "a, b, c"}, d...)...)
	slug := strings.TrimSpace(out)

	_, out, _ = runCLI(t, append([]string{"epic", "show", slug}, d...)...)
	if !strings.Contains(out, "tags:") || !strings.Contains(out, "a, b, c") {
		t.Errorf("tags round-trip: %q", out)
	}
}

func TestInvalidPriorityRejected(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	_, epicOut, _ := runCLI(t, append([]string{"epic", "add", "E"}, d...)...)
	_, issueOut, _ := runCLI(t, append([]string{"issue", "add", "I", "--epic", strings.TrimSpace(epicOut)}, d...)...)
	issue := strings.TrimSpace(issueOut)

	// The inlined priority check rejects a bad value at every add site.
	cases := [][]string{
		{"epic", "add", "Bad", "--priority", "bogus"},
		{"issue", "add", "Bad", "--priority", "bogus"},
		{"task", "add", "Bad", "--issue", issue, "--priority", "bogus"},
	}
	for _, c := range cases {
		code, _, errb := runCLI(t, append(c, d...)...)
		if code == 0 || !strings.Contains(errb, "invalid priority") {
			t.Errorf("%v with --priority bogus: code=%d err=%q", c[:2], code, errb)
		}
	}
}

func TestStatusAndPriority(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"epic", "add", "Web"}, d...)...)
	runCLI(t, append([]string{"issue", "add", "Roll", "--epic", "web"}, d...)...)
	runCLI(t, append([]string{"task", "add", "Do", "--issue", "roll"}, d...)...)

	// Setting status and priority round-trips through show.
	if code, _, errb := runCLI(t, append([]string{"task", "status", "do", "doing"}, d...)...); code != 0 {
		t.Fatalf("task status: %s", errb)
	}
	runCLI(t, append([]string{"task", "priority", "do", "high"}, d...)...)
	if _, out, _ := runCLI(t, append([]string{"task", "show", "do"}, d...)...); !strings.Contains(out, "doing") || !strings.Contains(out, "high") {
		t.Errorf("show after status/priority: %q", out)
	}

	// Invalid values are rejected, and the error names the allowed set.
	if code, _, errb := runCLI(t, append([]string{"task", "status", "do", "bogus"}, d...)...); code == 0 || !strings.Contains(errb, "invalid status") || !strings.Contains(errb, "todo") {
		t.Errorf("invalid status: code=%d err=%q", code, errb)
	}
	if code, _, errb := runCLI(t, append([]string{"epic", "priority", "web", "bogus"}, d...)...); code == 0 || !strings.Contains(errb, "invalid priority") || !strings.Contains(errb, "high") {
		t.Errorf("invalid priority: code=%d err=%q", code, errb)
	}

	// The resource acts as a type guard: a task slug is not an epic.
	if code, _, _ := runCLI(t, append([]string{"epic", "status", "do", "doing"}, d...)...); code == 0 {
		t.Error("epic status on a task slug should fail")
	}
}

// Setting an epic's priority must not freeze its rolled-up status: the epic
// still tracks its issues afterward.
func TestEpicPriorityKeepsStatusLive(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"epic", "add", "Web"}, d...)...)
	runCLI(t, append([]string{"issue", "add", "Roll", "--epic", "web"}, d...)...)

	// Set only priority, then move the issue to doing.
	runCLI(t, append([]string{"epic", "priority", "web", "high"}, d...)...)
	runCLI(t, append([]string{"issue", "status", "roll", "doing"}, d...)...)

	// The epic rolls up to doing rather than a frozen todo.
	if _, out, _ := runCLI(t, append([]string{"epic", "show", "web"}, d...)...); !strings.Contains(out, "doing") {
		t.Errorf("epic status should roll up to doing, got: %q", out)
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

// flagCmd builds a command carrying the persistent flags the resolvers read.
func flagCmd(dataDir, config string, global bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("data-dir", dataDir, "")
	cmd.Flags().String("config", config, "")
	cmd.Flags().Bool("global", global, "")
	return cmd
}

func TestDataDirResolution(t *testing.T) {
	t.Setenv("THING_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")

	// flag wins over everything.
	if got, _ := dataDir(flagCmd("/explicit", "", true)); got != "/explicit" {
		t.Errorf("flag: got %q", got)
	}
	// env beats -g.
	t.Setenv("THING_DATA_DIR", "/data/env")
	if got, _ := dataDir(flagCmd("", "", true)); got != "/data/env" {
		t.Errorf("env: got %q", got)
	}
	t.Setenv("THING_DATA_DIR", "")
	// -g resolves the global tree.
	home, _ := os.UserHomeDir()
	if got, _ := dataDir(flagCmd("", "", true)); got != filepath.Join(home, ".local", "share", "thing") {
		t.Errorf("-g: got %q", got)
	}

	// no flag/env/-g and no project -> error (no implicit global fallback).
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := dataDir(flagCmd("", "", false)); err == nil {
		t.Error("expected an error with no data directory")
	}
}

func TestConfigDirFallsBackToGlobal(t *testing.T) {
	t.Setenv("THING_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	if got, _ := configDir(flagCmd("", "", false)); got != filepath.Join(home, ".config", "thing") {
		t.Errorf("config fallback: got %q", got)
	}

	// config's flag and (separate) env var win over the fallback.
	if got, _ := configDir(flagCmd("", "/explicit", false)); got != "/explicit" {
		t.Errorf("config flag: got %q", got)
	}
	t.Setenv("THING_CONFIG_DIR", "/config/env")
	if got, _ := configDir(flagCmd("", "", false)); got != "/config/env" {
		t.Errorf("THING_CONFIG_DIR: got %q", got)
	}
}

// TestResolveInProject covers the in-project happy path for both resolvers and
// that -g overrides a discoverable project — branches the split left untested.
func TestResolveInProject(t *testing.T) {
	t.Setenv("THING_DATA_DIR", "")
	t.Setenv("THING_CONFIG_DIR", "")
	root := t.TempDir()
	// Resolve symlinks so macOS /var -> /private/var doesn't defeat the compare.
	root, _ = filepath.EvalSymlinks(root)
	thing := filepath.Join(root, ".thing")
	if err := os.MkdirAll(thing, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	// Inside a project, both data and config resolve to the discovered .thing/.
	if got, _ := dataDir(flagCmd("", "", false)); got != thing {
		t.Errorf("data in project: got %q, want %q", got, thing)
	}
	if got, _ := configDir(flagCmd("", "", false)); got != thing {
		t.Errorf("config in project: got %q, want %q", got, thing)
	}
	// -g overrides a discoverable project, for both resolvers.
	if got, _ := dataDir(flagCmd("", "", true)); got == thing {
		t.Errorf("-g should override the project dir for data, got %q", got)
	}
	if got, _ := configDir(flagCmd("", "", true)); got == thing {
		t.Errorf("-g should override the project dir for config, got %q", got)
	}
}

func TestInitAnchorsProjectDir(t *testing.T) {
	t.Setenv("THING_DATA_DIR", "")
	t.Setenv("THING_CONFIG_DIR", "")
	// bare init -> ./.thing
	if got, _ := initDataDir(flagCmd("", "", false)); got != ".thing" {
		t.Errorf("init data: got %q", got)
	}
	if got, _ := initConfigDir(flagCmd("", "", false)); got != ".thing" {
		t.Errorf("init config: got %q", got)
	}
	// -g targets the global directories instead, for both resolvers.
	home, _ := os.UserHomeDir()
	if got, _ := initDataDir(flagCmd("", "", true)); got != filepath.Join(home, ".local", "share", "thing") {
		t.Errorf("init -g data: got %q", got)
	}
	if got, _ := initConfigDir(flagCmd("", "", true)); got != filepath.Join(home, ".config", "thing") {
		t.Errorf("init -g config: got %q", got)
	}
}

func TestMvCommand(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"epic", "add", "Alpha"}, d...)...)
	runCLI(t, append([]string{"epic", "add", "Beta"}, d...)...)
	runCLI(t, append([]string{"issue", "add", "One", "--epic", "alpha"}, d...)...)

	// mv is silent on success (the destination is fully specified).
	if code, out, errb := runCLI(t, append([]string{"mv", "alpha/one", "beta/planning"}, d...)...); code != 0 || out != "" {
		t.Fatalf("mv: code=%d out=%q err=%q", code, out, errb)
	}
	// The move+rename is reflected: new slug under the new parent.
	if _, out, _ := runCLI(t, append([]string{"tree"}, d...)...); !strings.Contains(out, "planning") || strings.Contains(out, "(one)") {
		t.Errorf("tree after mv: %q", out)
	}
	if code, _, _ := runCLI(t, append([]string{"issue", "show", "planning"}, d...)...); code != 0 {
		t.Error("issue show planning should succeed after mv")
	}
	// A bad source path errors.
	if code, _, _ := runCLI(t, append([]string{"mv", "alpha/planning", "beta/x"}, d...)...); code == 0 {
		t.Error("mv from a wrong parent path should fail")
	}
}
