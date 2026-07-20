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

	// add an epic (a bare title, no parent); the printed path is the epic slug
	code, out, errb := runCLI(t, append([]string{"add", "Web release"}, d...)...)
	if code != 0 {
		t.Fatalf("add epic: code=%d err=%s", code, errb)
	}
	epicPath := strings.TrimSpace(out)
	if epicPath != "web-release" {
		t.Fatalf("epic path = %q, want web-release", epicPath)
	}

	// add an issue under the epic; the printed path is <epic>/<issue>
	_, out, errb = runCLI(t, append([]string{"add", epicPath + "/Monitor rollout"}, d...)...)
	issuePath := strings.TrimSpace(out)
	if issuePath != "web-release/monitor-rollout" {
		t.Fatalf("issue path = %q err=%s", issuePath, errb)
	}

	// adding under a nonexistent parent fails
	if code, _, _ := runCLI(t, append([]string{"add", "ghost/nope"}, d...)...); code == 0 {
		t.Fatal("add under a nonexistent parent should fail")
	}

	// add a task under the issue; the printed path is <epic>/<issue>/<task>
	_, out, _ = runCLI(t, append([]string{"add", issuePath + "/Confirm routing"}, d...)...)
	taskPath := strings.TrimSpace(out)
	if taskPath != "web-release/monitor-rollout/confirm-routing" {
		t.Fatalf("task path = %q", taskPath)
	}

	// a task cannot be a parent (the hierarchy stops at tasks)
	if code, _, errb := runCLI(t, append([]string{"add", taskPath + "/nope"}, d...)...); code == 0 {
		t.Fatalf("add under a task should fail; errb=%s", errb)
	}

	// add an orphan issue (under _orphan)
	_, out, _ = runCLI(t, append([]string{"add", "_orphan/Loose end"}, d...)...)
	if strings.TrimSpace(out) != "_orphan/loose-end" {
		t.Fatalf("orphan path = %q", out)
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
	_, out1, _ := runCLI(t, append([]string{"add", "Same Name"}, d...)...)
	_, out2, _ := runCLI(t, append([]string{"add", "Same Name"}, d...)...)
	if strings.TrimSpace(out1) != "same-name" || strings.TrimSpace(out2) != "same-name-2" {
		t.Errorf("uniqueness: %q then %q", out1, out2)
	}
}

func TestLsAndShow(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)

	// ls with no arg lists the top level; ls <parent> lists its children.
	if _, out, _ := runCLI(t, append([]string{"ls"}, d...)...); !strings.Contains(out, "web") {
		t.Errorf("ls: %q", out)
	}
	if _, out, _ := runCLI(t, append([]string{"ls", "web/roll"}, d...)...); !strings.Contains(out, "do") {
		t.Errorf("ls web/roll: %q", out)
	}

	// ls _orphan lists orphan issues and excludes epics.
	runCLI(t, append([]string{"add", "_orphan/Loose"}, d...)...)
	if _, out, _ := runCLI(t, append([]string{"ls", "_orphan"}, d...)...); !strings.Contains(out, "loose") || strings.Contains(out, "web") {
		t.Errorf("ls _orphan: %q", out)
	}
	// ls of an unknown parent errors.
	if code, _, _ := runCLI(t, append([]string{"ls", "nope"}, d...)...); code == 0 {
		t.Error("ls on an unknown parent should fail")
	}

	// show reflects any node by its full path.
	if _, out, _ := runCLI(t, append([]string{"show", "web/roll/do"}, d...)...); !strings.Contains(out, "Do") {
		t.Errorf("show: %q", out)
	}
	if code, _, _ := runCLI(t, append([]string{"show", "nope"}, d...)...); code == 0 {
		t.Error("show on an unknown slug should fail")
	}
}

func TestAddCategoryOnlyForEpic(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)

	// --category is fine on an epic (no parent).
	if code, _, errb := runCLI(t, append([]string{"add", "Other", "--category", "Proj"}, d...)...); code != 0 {
		t.Fatalf("add epic --category: %s", errb)
	}
	// --category with a parent (a non-epic) is rejected.
	if code, _, errb := runCLI(t, append([]string{"add", "web/Sub", "--category", "Proj"}, d...)...); code == 0 || !strings.Contains(errb, "category") {
		t.Errorf("add issue --category: code=%d err=%q", code, errb)
	}
	// A leading slash (an empty parent) is rejected.
	if code, _, _ := runCLI(t, append([]string{"add", "/Nope"}, d...)...); code == 0 {
		t.Error("add with a leading slash should fail")
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
	_, out, _ := runCLI(t, append([]string{"add", "Tagged", "--tags", "a, b, c"}, d...)...)
	slug := strings.TrimSpace(out)

	_, out, _ = runCLI(t, append([]string{"show", slug}, d...)...)
	if !strings.Contains(out, "tags:") || !strings.Contains(out, "a, b, c") {
		t.Errorf("tags round-trip: %q", out)
	}
}

func TestInvalidPriorityRejected(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "E"}, d...)...)
	runCLI(t, append([]string{"add", "e/I"}, d...)...)

	// A bad --priority is rejected whatever the target's type (epic/issue/task).
	cases := []string{"Bad", "e/Bad", "i/Bad"}
	for _, path := range cases {
		code, _, errb := runCLI(t, append([]string{"add", path, "--priority", "bogus"}, d...)...)
		if code == 0 || !strings.Contains(errb, "invalid priority") {
			t.Errorf("add %q --priority bogus: code=%d err=%q", path, code, errb)
		}
	}
}

func TestStatusAndPriority(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)

	// status/priority take a node's full path and round-trip.
	if code, _, errb := runCLI(t, append([]string{"status", "web/roll/do", "doing"}, d...)...); code != 0 {
		t.Fatalf("status: %s", errb)
	}
	runCLI(t, append([]string{"priority", "web/roll/do", "high"}, d...)...)
	if _, out, _ := runCLI(t, append([]string{"show", "web/roll/do"}, d...)...); !strings.Contains(out, "doing") || !strings.Contains(out, "high") {
		t.Errorf("show after status/priority: %q", out)
	}

	// Invalid values are rejected, and the error names the allowed set.
	if code, _, errb := runCLI(t, append([]string{"status", "web/roll/do", "bogus"}, d...)...); code == 0 || !strings.Contains(errb, "invalid status") || !strings.Contains(errb, "todo") {
		t.Errorf("invalid status: code=%d err=%q", code, errb)
	}
	if code, _, errb := runCLI(t, append([]string{"priority", "web", "bogus"}, d...)...); code == 0 || !strings.Contains(errb, "invalid priority") || !strings.Contains(errb, "high") {
		t.Errorf("invalid priority: code=%d err=%q", code, errb)
	}

	// An unknown path is rejected.
	if code, _, _ := runCLI(t, append([]string{"status", "nope", "doing"}, d...)...); code == 0 {
		t.Error("status on an unknown path should fail")
	}
}

// Setting an epic's priority must not freeze its rolled-up status: the epic
// still tracks its issues afterward.
func TestEpicPriorityKeepsStatusLive(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)

	// Set only priority, then move the issue to doing.
	runCLI(t, append([]string{"priority", "web", "high"}, d...)...)
	runCLI(t, append([]string{"status", "web/roll", "doing"}, d...)...)

	// The epic rolls up to doing rather than a frozen todo.
	if _, out, _ := runCLI(t, append([]string{"show", "web"}, d...)...); !strings.Contains(out, "doing") {
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
	runCLI(t, append([]string{"add", "Alpha"}, d...)...)
	runCLI(t, append([]string{"add", "Beta"}, d...)...)
	runCLI(t, append([]string{"add", "alpha/One"}, d...)...)

	// mv is silent on success (the destination is fully specified).
	if code, out, errb := runCLI(t, append([]string{"mv", "alpha/one", "beta/planning"}, d...)...); code != 0 || out != "" {
		t.Fatalf("mv: code=%d out=%q err=%q", code, out, errb)
	}
	// The move+rename is reflected: new slug under the new parent.
	if _, out, _ := runCLI(t, append([]string{"tree"}, d...)...); !strings.Contains(out, "planning") || strings.Contains(out, "(one)") {
		t.Errorf("tree after mv: %q", out)
	}
	if code, _, _ := runCLI(t, append([]string{"show", "beta/planning"}, d...)...); code != 0 {
		t.Error("show beta/planning should succeed after mv")
	}
	// The old path no longer resolves.
	if code, _, _ := runCLI(t, append([]string{"show", "alpha/one"}, d...)...); code == 0 {
		t.Error("the old path alpha/one should no longer resolve")
	}
}

func TestRmCommand(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)

	// rm is silent on success; a task removal leaves its issue.
	if code, out, errb := runCLI(t, append([]string{"rm", "web/roll/do"}, d...)...); code != 0 || out != "" {
		t.Fatalf("rm task: code=%d out=%q err=%q", code, out, errb)
	}
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll/do"}, d...)...); code == 0 {
		t.Error("task should be gone")
	}
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll"}, d...)...); code != 0 {
		t.Error("issue 'web/roll' should survive removing its task")
	}

	// Removing the epic takes its subtree.
	runCLI(t, append([]string{"rm", "web"}, d...)...)
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll"}, d...)...); code == 0 {
		t.Error("issue 'web/roll' should be gone with its epic")
	}

	// Removing an unknown path errors.
	if code, _, _ := runCLI(t, append([]string{"rm", "nope"}, d...)...); code == 0 {
		t.Error("rm on an unknown path should fail")
	}
}

func TestLinkCommand(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)

	// list on a node with no links prints nothing and succeeds.
	if code, out, _ := runCLI(t, append([]string{"link", "list", "web"}, d...)...); code != 0 || out != "" {
		t.Errorf("link list (empty): code=%d out=%q", code, out)
	}

	// add and rm are silent; list prints a numbered list.
	if code, out, errb := runCLI(t, append([]string{"link", "add", "web", "https://x", "--label", "X"}, d...)...); code != 0 || out != "" {
		t.Fatalf("link add: code=%d out=%q err=%q", code, out, errb)
	}
	runCLI(t, append([]string{"link", "add", "web", "https://y"}, d...)...)
	if _, out, _ := runCLI(t, append([]string{"link", "list", "web"}, d...)...); !strings.Contains(out, "1. https://x (X)") || !strings.Contains(out, "2. https://y") {
		t.Errorf("link list: %q", out)
	}

	// Remove by index, then confirm.
	runCLI(t, append([]string{"link", "rm", "web", "1"}, d...)...)
	if _, out, _ := runCLI(t, append([]string{"link", "list", "web"}, d...)...); strings.Contains(out, "https://x") || !strings.Contains(out, "https://y") {
		t.Errorf("link list after rm: %q", out)
	}

	// An unknown ref errors.
	if code, _, _ := runCLI(t, append([]string{"link", "add", "nope", "https://z"}, d...)...); code == 0 {
		t.Error("link add on an unknown ref should fail")
	}
}

func TestTreeGroupsByCategory(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	// Configure category headings and their order.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("title: Board\ncategories:\n  - Project\n  - Personal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCLI(t, append([]string{"add", "Web", "--category", "Project"}, d...)...)
	runCLI(t, append([]string{"add", "Home", "--category", "Personal"}, d...)...)
	runCLI(t, append([]string{"add", "Misc"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "_orphan/Loose"}, d...)...)

	// tree and top-level ls both group under headings in config order.
	for _, cmd := range [][]string{{"tree"}, {"ls"}} {
		_, out, _ := runCLI(t, append(cmd, d...)...)
		i, j, k := strings.Index(out, "# Project"), strings.Index(out, "# Personal"), strings.Index(out, "# (uncategorized)")
		if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
			t.Errorf("%v grouping: Project=%d Personal=%d uncat=%d\n%s", cmd, i, j, k, out)
		}
		if !strings.Contains(out[k:], "misc") {
			t.Errorf("%v: misc should be uncategorized\n%s", cmd, out)
		}
	}

	// Grouping applies only to the top level: ls of a parent or _orphan is flat.
	for _, cmd := range [][]string{{"ls", "web"}, {"ls", "_orphan"}} {
		if _, out, _ := runCLI(t, append(cmd, d...)...); strings.Contains(out, "#") {
			t.Errorf("%v should be flat (no headings): %q", cmd, out)
		}
	}
}
