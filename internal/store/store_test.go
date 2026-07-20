package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
)

// write creates a node file at the given path, making parent dirs as needed.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Epic "alpha" with two issues.
	write(t, filepath.Join(root, "alpha", "_epic.md"), "---\ntitle: Alpha\npriority: high\n---\nEpic body\n")
	write(t, filepath.Join(root, "alpha", "one", "_issue.md"), "---\ntitle: One\nstatus: done\n---\n")
	write(t, filepath.Join(root, "alpha", "one", "task-a.md"), "---\ntitle: Task A\nstatus: done\n---\n")
	write(t, filepath.Join(root, "alpha", "two", "_issue.md"), "---\ntitle: Two\nstatus: doing\n---\n")
	// Orphan issue.
	write(t, filepath.Join(root, OrphanDir, "loose", "_issue.md"), "---\ntitle: Loose\n---\n")
	write(t, filepath.Join(root, OrphanDir, "loose", "z-task.md"), "---\ntitle: Z\n---\n")
	write(t, filepath.Join(root, OrphanDir, "loose", "a-task.md"), "---\ntitle: A\n---\n")
	// Non-node clutter that must be ignored.
	write(t, filepath.Join(root, "config.yaml"), "title: Test\n")
	return root
}

func TestLoad(t *testing.T) {
	root := fixture(t)
	top, err := Open(root).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Top level sorted by slug: "alpha" (epic) then "loose" (orphan issue).
	if len(top) != 2 {
		t.Fatalf("top-level count = %d, want 2", len(top))
	}
	epic := top[0]
	if epic.Type != model.Epic || epic.Slug != "alpha" || epic.Title != "Alpha" {
		t.Errorf("epic = %+v", epic)
	}
	if len(epic.Children) != 2 {
		t.Fatalf("epic children = %d, want 2", len(epic.Children))
	}
	if epic.Children[0].Slug != "one" || epic.Children[1].Slug != "two" {
		t.Errorf("issue order = %s, %s", epic.Children[0].Slug, epic.Children[1].Slug)
	}
	if got := epic.Children[0].Children; len(got) != 1 || got[0].Slug != "task-a" || got[0].Type != model.Task {
		t.Errorf("task load = %+v", got)
	}

	orphan := top[1]
	if orphan.Type != model.Issue || orphan.Slug != "loose" {
		t.Errorf("orphan = %+v", orphan)
	}
	// Tasks sorted by slug.
	if len(orphan.Children) != 2 || orphan.Children[0].Slug != "a-task" || orphan.Children[1].Slug != "z-task" {
		t.Errorf("orphan tasks = %+v", orphan.Children)
	}
	// A file that omits status loads with an empty Status; the todo default is
	// a display-time derivation, not baked into the loaded node.
	if orphan.Status != "" {
		t.Errorf("orphan raw status = %q, want empty", orphan.Status)
	}
	if orphan.EffectiveStatus() != model.Todo {
		t.Errorf("orphan effective status = %q, want todo", orphan.EffectiveStatus())
	}
}

func TestRollup(t *testing.T) {
	root := fixture(t)
	top, _ := Open(root).Load()
	// alpha omits its own status; one=done and two=doing -> any doing -> doing.
	// The rollup is a display derivation, so the loaded node's Status stays empty.
	if top[0].Status != "" {
		t.Errorf("alpha raw status = %q, want empty", top[0].Status)
	}
	if top[0].EffectiveStatus() != model.Doing {
		t.Errorf("alpha rollup = %q, want doing", top[0].EffectiveStatus())
	}
}

func TestRollupExplicitWins(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "e", "_epic.md"), "---\ntitle: E\nstatus: paused\n---\n")
	write(t, filepath.Join(root, "e", "i", "_issue.md"), "---\ntitle: I\nstatus: doing\n---\n")
	top, _ := Open(root).Load()
	if top[0].EffectiveStatus() != model.Paused {
		t.Errorf("explicit epic status = %q, want paused", top[0].EffectiveStatus())
	}
}

func TestGlobalDirs(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	if got, _ := GlobalDataDir(); got != "/xdg/data/thing" {
		t.Errorf("XDG_DATA_HOME: got %q", got)
	}
	if got, _ := GlobalConfigDir(); got != "/xdg/config/thing" {
		t.Errorf("XDG_CONFIG_HOME: got %q", got)
	}

	// No XDG vars -> ~/.local/share/thing and ~/.config/thing.
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()
	if got, _ := GlobalDataDir(); got != filepath.Join(home, ".local", "share", "thing") {
		t.Errorf("data default: got %q", got)
	}
	if got, _ := GlobalConfigDir(); got != filepath.Join(home, ".config", "thing") {
		t.Errorf("config default: got %q", got)
	}
}

func TestFindProjectDir(t *testing.T) {
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
	if got, ok := FindProjectDir(); !ok || got != thing {
		t.Errorf("upward search: got %q ok=%v, want %q", got, ok, thing)
	}
}

// TestIndexPaths checks that nodes are keyed by their full slug-path.
func TestIndexPaths(t *testing.T) {
	idx, err := Open(fixture(t)).Index()
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	for _, path := range []string{"alpha", "alpha/one", "alpha/two", "alpha/one/task-a", "_orphan/loose", "_orphan/loose/a-task"} {
		if idx[path] == nil {
			t.Errorf("missing path %q in index", path)
		}
	}
	// A bare slug is not a valid key when it is not top-level.
	if idx["task-a"] != nil || idx["one"] != nil {
		t.Error("nodes should be keyed by full path, not bare slug")
	}
	if got := idx["alpha/one/task-a"]; got != nil && got.Parent != "alpha/one" {
		t.Errorf("task parent path = %q, want alpha/one", got.Parent)
	}
}

// TestSiblingScopedSlugs checks that a slug is unique only among its siblings:
// the same title recurs verbatim under different parents, but auto-numbers when
// repeated under one parent.
func TestSiblingScopedSlugs(t *testing.T) {
	s := Open(t.TempDir())
	for _, title := range []string{"A", "B"} {
		if _, err := s.Add("", &model.Node{Title: title}); err != nil {
			t.Fatalf("Add epic %q: %v", title, err)
		}
	}
	ra, err := s.Add("a", &model.Node{Title: "Review"})
	if err != nil {
		t.Fatal(err)
	}
	rb, err := s.Add("b", &model.Node{Title: "Review"})
	if err != nil {
		t.Fatal(err)
	}
	if ra != "a/review" || rb != "b/review" {
		t.Errorf("same name under different parents = %q, %q; want a/review, b/review", ra, rb)
	}
	// A second "Review" under the same parent auto-numbers.
	if r2, _ := s.Add("a", &model.Node{Title: "Review"}); r2 != "a/review-2" {
		t.Errorf("same name under one parent = %q, want a/review-2", r2)
	}
}

func TestSave(t *testing.T) {
	s := Open(fixture(t))

	// A task's status write round-trips through disk, along with its updated date.
	loc, _ := s.Locate("alpha/one/task-a")
	loc.Node.Status = model.Doing
	loc.Node.Updated = "2026-07-20"
	if err := s.Save(loc); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, _ := s.Locate("alpha/one/task-a")
	if reloaded.Node.Status != model.Doing || reloaded.Node.Updated != "2026-07-20" {
		t.Errorf("reloaded = {status:%q updated:%q}", reloaded.Node.Status, reloaded.Node.Updated)
	}

	// alpha's epic file omits its own status (rollup derives "doing"). Saving a
	// non-status field must not freeze that derived status onto the file, and
	// must not lose the node's body.
	epic, _ := s.Locate("alpha")
	if epic.Node.Status != "" {
		t.Fatalf("precondition: alpha raw status = %q, want empty", epic.Node.Status)
	}
	epic.Node.Priority = model.Low
	if err := s.Save(epic); err != nil {
		t.Fatalf("Save epic priority: %v", err)
	}
	after, _ := s.Locate("alpha")
	if after.Node.Status != "" {
		t.Errorf("epic status frozen to %q after a priority-only save; want it left empty", after.Node.Status)
	}
	if after.Node.EffectiveStatus() != model.Doing {
		t.Errorf("epic effective status = %q, want doing (rollup must stay live)", after.Node.EffectiveStatus())
	}
	if after.Node.Body == "" {
		t.Error("epic body lost after Save")
	}

	// An explicitly set epic status, by contrast, does persist across reload.
	after.Node.Status = model.Paused
	if err := s.Save(after); err != nil {
		t.Fatalf("Save epic status: %v", err)
	}
	if got, _ := s.Locate("alpha"); got.Node.Status != model.Paused {
		t.Errorf("epic status = %q, want paused (an explicit status must persist)", got.Node.Status)
	}
}

func TestFind(t *testing.T) {
	s := Open(fixture(t))

	// Found with the matching type.
	loc, err := s.Find("alpha", model.Epic)
	if err != nil {
		t.Fatalf("Find(alpha, Epic): %v", err)
	}
	if loc.Node.Slug != "alpha" {
		t.Errorf("Find(alpha, Epic) = %q", loc.Node.Slug)
	}

	// Found, but the type guard rejects it.
	if _, err := s.Find("alpha", model.Task); err == nil {
		t.Error("Find(alpha, Task) should fail: alpha is an epic")
	}

	// Missing slug.
	if _, err := s.Find("ghost", model.Task); err == nil {
		t.Error("Find(ghost, Task) should fail: no such slug")
	}
}

// mvFixture builds: epic alpha (issue one -> task t), epic beta, and an orphan
// issue "ref" whose body links to [[alpha/one]] by path.
func mvFixture(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "alpha", "_epic.md"), "---\ntitle: Alpha\n---\n")
	write(t, filepath.Join(root, "alpha", "one", "_issue.md"), "---\ntitle: One\n---\n")
	write(t, filepath.Join(root, "alpha", "one", "t.md"), "---\ntitle: T\n---\n")
	write(t, filepath.Join(root, "beta", "_epic.md"), "---\ntitle: Beta\n---\n")
	// The ref issue links to alpha/one, to its descendant task alpha/one/t, and
	// to a decoy that merely shares the "alpha/one" prefix (must not be touched).
	write(t, filepath.Join(root, OrphanDir, "ref", "_issue.md"),
		"---\ntitle: Ref\n---\nsee [[alpha/one]], [[alpha/one/t]], [[alpha/one-thing]]\n")
	return Open(root)
}

func TestMvRename(t *testing.T) {
	s := mvFixture(t)
	if err := s.Mv("alpha/one", "alpha/planning", "2026-07-20"); err != nil {
		t.Fatalf("Mv rename: %v", err)
	}
	// The slug changed; the old path is gone and the new one is under the same epic.
	if loc, _ := s.Locate("alpha/one"); loc != nil {
		t.Error("old path 'alpha/one' still resolves after rename")
	}
	loc, _ := s.Locate("alpha/planning")
	if loc == nil || loc.Parent != "alpha" {
		t.Fatalf("planning not found under alpha: %+v", loc)
	}
	// The title is untouched; only the slug (the name) changed.
	if loc.Node.Title != "One" {
		t.Errorf("title = %q, want One (mv must not change the title)", loc.Node.Title)
	}
	// The child task moved with its issue (its path prefix changed).
	if task, _ := s.Locate("alpha/planning/t"); task == nil || task.Parent != "alpha/planning" {
		t.Errorf("task did not follow its issue: %+v", task)
	}
	// Backlinks (refs) follow the rename: the node's own ref and its descendant
	// task's ref are rewritten, but a prefix-sharing decoy is left alone.
	ref, _ := s.Locate("_orphan/ref")
	if ref == nil {
		t.Fatal("ref issue missing")
	}
	if !strings.Contains(ref.Node.Body, "[[alpha/planning]]") {
		t.Errorf("own backlink not rewritten: %q", ref.Node.Body)
	}
	if !strings.Contains(ref.Node.Body, "[[alpha/planning/t]]") {
		t.Errorf("descendant backlink not rewritten: %q", ref.Node.Body)
	}
	if !strings.Contains(ref.Node.Body, "[[alpha/one-thing]]") {
		t.Errorf("prefix-sharing decoy was wrongly rewritten: %q", ref.Node.Body)
	}
}

func TestMvMove(t *testing.T) {
	s := mvFixture(t)
	if err := s.Mv("alpha/one", "beta/one", "2026-07-20"); err != nil {
		t.Fatalf("Mv move: %v", err)
	}
	loc, _ := s.Locate("beta/one")
	if loc == nil || loc.Parent != "beta" {
		t.Fatalf("one not moved under beta: %+v", loc)
	}
	// A move changes the path, so path backlinks follow: [[alpha/one]] -> [[beta/one]].
	if ref, _ := s.Locate("_orphan/ref"); ref == nil || !strings.Contains(ref.Node.Body, "[[beta/one]]") {
		t.Errorf("backlink not rewritten on move: %q", ref.Node.Body)
	}
}

func TestMvMoveAndRename(t *testing.T) {
	s := mvFixture(t)
	if err := s.Mv("alpha/one", "beta/roadmap", "2026-07-20"); err != nil {
		t.Fatalf("Mv both: %v", err)
	}
	loc, _ := s.Locate("beta/roadmap")
	if loc == nil || loc.Parent != "beta" {
		t.Fatalf("roadmap not under beta: %+v", loc)
	}
	if ref, _ := s.Locate("_orphan/ref"); ref == nil || !strings.Contains(ref.Node.Body, "[[beta/roadmap]]") {
		t.Errorf("backlink not rewritten on move+rename: %q", ref.Node.Body)
	}
}

func TestMvToOrphanAndTaskAndEpic(t *testing.T) {
	s := mvFixture(t)
	// Issue to _orphan.
	if err := s.Mv("alpha/one", "_orphan/one", "2026-07-20"); err != nil {
		t.Fatalf("Mv to orphan: %v", err)
	}
	if loc, _ := s.Locate("_orphan/one"); loc == nil || loc.Parent != OrphanDir {
		t.Errorf("one not an orphan issue: %+v", loc)
	}
	// Task to another issue (create a second issue first).
	write(t, filepath.Join(s.Root, "beta", "two", "_issue.md"), "---\ntitle: Two\n---\n")
	if err := s.Mv("_orphan/one/t", "beta/two/t", "2026-07-20"); err != nil {
		t.Fatalf("Mv task: %v", err)
	}
	if task, _ := s.Locate("beta/two/t"); task == nil || task.Parent != "beta/two" {
		t.Errorf("task not moved to beta/two: %+v", task)
	}
	// Epic rename (a bare name; epics have no parent).
	if err := s.Mv("alpha", "gamma", "2026-07-20"); err != nil {
		t.Fatalf("Mv epic rename: %v", err)
	}
	if loc, _ := s.Locate("gamma"); loc == nil || loc.Node.Type != model.Epic {
		t.Errorf("epic not renamed to gamma: %+v", loc)
	}
}

func TestMvErrors(t *testing.T) {
	s := mvFixture(t)
	// Source path does not exist (one is under alpha, not beta).
	if err := s.Mv("beta/one", "beta/x", "2026-07-20"); err == nil {
		t.Error("expected an error: no node at 'beta/one'")
	}
	// The same ref is a no-op, not an error.
	if err := s.Mv("alpha/one", "alpha/one", "2026-07-20"); err != nil {
		t.Errorf("same ref should be a no-op, not an error: %v", err)
	}
	// Renaming onto an existing sibling slug errors.
	write(t, filepath.Join(s.Root, "alpha", "two", "_issue.md"), "---\ntitle: Two\n---\n")
	if err := s.Mv("alpha/one", "alpha/two", "2026-07-20"); err == nil {
		t.Error("expected an error: slug 'two' already exists under alpha")
	}
	// An epic cannot be given a parent.
	if err := s.Mv("alpha", "beta/alpha", "2026-07-20"); err == nil {
		t.Error("expected an error: an epic has no parent")
	}
	// Moving an issue under a nonexistent epic.
	if err := s.Mv("alpha/one", "ghost/one", "2026-07-20"); err == nil {
		t.Error("expected an error: no such epic 'ghost'")
	}
	// Moving a task under a nonexistent issue.
	if err := s.Mv("alpha/one/t", "ghost/t", "2026-07-20"); err == nil {
		t.Error("expected an error: no such issue 'ghost'")
	}
	// An empty source or destination.
	if err := s.Mv("", "beta/x", "2026-07-20"); err == nil {
		t.Error("expected an error: empty source")
	}
	if err := s.Mv("alpha/one", "", "2026-07-20"); err == nil {
		t.Error("expected an error: empty destination")
	}
}

// The destination name is slugified, and that normalized slug is what the node
// is filed under and used for the backlink path.
func TestMvSlugifiesDest(t *testing.T) {
	s := mvFixture(t)
	if err := s.Mv("alpha/one", "alpha/Q3 Roadmap", "2026-07-20"); err != nil {
		t.Fatalf("Mv: %v", err)
	}
	loc, _ := s.Locate("alpha/q3-roadmap")
	if loc == nil || loc.Parent != "alpha" {
		t.Fatalf("node not filed under normalized slug: %+v", loc)
	}
	if ref, _ := s.Locate("_orphan/ref"); ref == nil || !strings.Contains(ref.Node.Body, "[[alpha/q3-roadmap]]") {
		t.Errorf("backlink not rewritten to the normalized path: %q", ref.Node.Body)
	}
}

// Moving a node onto itself is a no-op that does not touch the file, so its
// updated date is left alone.
func TestMvNoOp(t *testing.T) {
	s := mvFixture(t)
	before, _ := os.ReadFile(filepath.Join(s.Root, "alpha", "one", "_issue.md"))
	if err := s.Mv("alpha/one", "alpha/one", "2099-01-01"); err != nil {
		t.Fatalf("Mv no-op: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, "alpha", "one", "_issue.md"))
	if string(before) != string(after) {
		t.Errorf("no-op mv rewrote the file:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestRemove(t *testing.T) {
	s := Open(fixture(t))

	// Removing a task deletes only its file; its issue survives.
	task, _ := s.Locate("alpha/one/task-a")
	if err := s.Remove(task); err != nil {
		t.Fatalf("Remove task: %v", err)
	}
	if loc, _ := s.Locate("alpha/one/task-a"); loc != nil {
		t.Error("task still resolves after Remove")
	}
	if loc, _ := s.Locate("alpha/one"); loc == nil {
		t.Error("issue 'alpha/one' should survive removing its task")
	}

	// Removing an issue deletes its whole subtree; a sibling issue survives.
	issue, _ := s.Locate("alpha/one")
	if err := s.Remove(issue); err != nil {
		t.Fatalf("Remove issue: %v", err)
	}
	if loc, _ := s.Locate("alpha/one"); loc != nil {
		t.Error("issue 'alpha/one' still resolves after Remove")
	}
	if loc, _ := s.Locate("alpha/two"); loc == nil {
		t.Error("sibling issue 'alpha/two' should survive")
	}

	// Removing an epic takes its subtree; an orphan issue elsewhere survives.
	epic, _ := s.Locate("alpha")
	if err := s.Remove(epic); err != nil {
		t.Fatalf("Remove epic: %v", err)
	}
	if loc, _ := s.Locate("alpha/two"); loc != nil {
		t.Error("issue 'alpha/two' should be gone with its epic")
	}
	if loc, _ := s.Locate("_orphan/loose"); loc == nil {
		t.Error("orphan issue '_orphan/loose' should survive")
	}
}

func TestLinks(t *testing.T) {
	s := Open(fixture(t))
	reload := func() *Entry { e, _ := s.Locate("alpha/one"); return e }

	if err := s.AddLink(reload(), "https://a", "A", "2026-07-20"); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	if err := s.AddLink(reload(), "https://b", "", "2026-07-20"); err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	e := reload()
	if len(e.Node.Links) != 2 || e.Node.Updated != "2026-07-20" {
		t.Fatalf("after adds: links=%+v updated=%q", e.Node.Links, e.Node.Updated)
	}

	// Adding a duplicate URL updates its label (and re-stamps updated) rather
	// than appending.
	if err := s.AddLink(reload(), "https://a", "A2", "2026-07-21"); err != nil {
		t.Fatal(err)
	}
	if e := reload(); len(e.Node.Links) != 2 || e.Node.Links[0].Label != "A2" || e.Node.Updated != "2026-07-21" {
		t.Errorf("dup add: links=%+v updated=%q", e.Node.Links, e.Node.Updated)
	}
	// Re-adding with an empty label clears the existing one.
	s.AddLink(reload(), "https://a", "", "2026-07-21")
	if e := reload(); e.Node.Links[0].Label != "" {
		t.Errorf("empty-label add did not clear the label: %+v", e.Node.Links[0])
	}

	// Remove by URL, then by 1-based index.
	if err := s.RemoveLink(reload(), "https://a", "2026-07-21"); err != nil {
		t.Fatal(err)
	}
	if e := reload(); len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://b" {
		t.Errorf("rm by url: %+v", e.Node.Links)
	}
	if err := s.RemoveLink(reload(), "1", "2026-07-21"); err != nil {
		t.Fatal(err)
	}
	if e := reload(); len(e.Node.Links) != 0 {
		t.Errorf("rm by index: %+v", e.Node.Links)
	}

	// An out-of-range index and a non-matching ref both error.
	if err := s.RemoveLink(reload(), "1", "x"); err == nil {
		t.Error("expected out-of-range error")
	}
	if err := s.RemoveLink(reload(), "https://nope", "x"); err == nil {
		t.Error("expected no-match error")
	}
}

// A link whose URL is numeric is still matched by URL before the index
// fallback, so RemoveLink("1") deletes the URL-"1" link, not the first by index.
func TestRemoveLinkURLBeatsIndex(t *testing.T) {
	s := Open(fixture(t))
	e, _ := s.Locate("alpha/one")
	s.AddLink(e, "https://first", "", "2026-07-20")
	e, _ = s.Locate("alpha/one")
	s.AddLink(e, "1", "", "2026-07-20") // a link whose URL is literally "1"
	e, _ = s.Locate("alpha/one")
	if err := s.RemoveLink(e, "1", "2026-07-20"); err != nil {
		t.Fatal(err)
	}
	e, _ = s.Locate("alpha/one")
	if len(e.Node.Links) != 1 || e.Node.Links[0].URL != "https://first" {
		t.Errorf("URL match should win over index: %+v", e.Node.Links)
	}
}
