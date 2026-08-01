package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
)

// Archiving a task moves just its file under _archives/, records where it came
// from, and drops it from the live tree.
func TestArchiveTask(t *testing.T) {
	s := Open(fixture(t))
	task, _ := s.Get("alpha/one/task-a")
	ref, err := s.Archive(task, "2026-07-27T09:00:00Z")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if ref != "_archives/task-a" {
		t.Errorf("archive ref = %q, want _archives/task-a", ref)
	}
	// Gone from the live tree.
	if loc, _ := s.Locate("alpha/one/task-a"); loc != nil {
		t.Error("archived task still resolves in the live tree")
	}
	// Present in the archive, with its origin recorded.
	ae, err := s.ArchiveGet("_archives/task-a")
	if err != nil {
		t.Fatalf("ArchiveGet: %v", err)
	}
	if ae.Node.Type != model.Task {
		t.Errorf("archived type = %q, want task", ae.Node.Type)
	}
	if ae.Node.ArchivedRef != "alpha/one/task-a" || ae.Node.ArchivedAt != "2026-07-27T09:00:00Z" {
		t.Errorf("archive metadata = {from:%q at:%q}", ae.Node.ArchivedRef, ae.Node.ArchivedAt)
	}
}

// Archiving an issue takes its whole subtree (its directory), so its tasks go
// with it.
func TestArchiveIssueSubtree(t *testing.T) {
	s := Open(fixture(t))
	issue, _ := s.Get("alpha/one")
	if _, err := s.Archive(issue, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatalf("Archive issue: %v", err)
	}
	if loc, _ := s.Locate("alpha/one"); loc != nil {
		t.Error("archived issue still in the live tree")
	}
	// The task file rode along inside the archived directory.
	if _, err := os.Stat(filepath.Join(s.Root, ArchiveDir, "one", "task-a.md")); err != nil {
		t.Errorf("child task did not move with its issue: %v", err)
	}
	// The sibling issue survives.
	if loc, _ := s.Locate("alpha/two"); loc == nil {
		t.Error("sibling issue alpha/two should survive")
	}
}

// A failed relocation into _archives must not leave archive metadata on the
// still-live node. Archive moves the file first and stamps the metadata at the
// destination, so a rename error leaves the origin untouched and retryable.
func TestArchiveRenameFailureLeavesLiveNodeClean(t *testing.T) {
	s := Open(fixture(t))
	// Block the destination: a non-empty directory sits where the task's archive
	// file (_archives/task-a.md) would land, so os.Rename into it fails.
	write(t, filepath.Join(s.Root, ArchiveDir, "task-a.md", "blocker"), "x")

	task, _ := s.Get("alpha/one/task-a")
	if _, err := s.Archive(task, "2026-07-27T09:00:00Z"); err == nil {
		t.Fatal("expected Archive to fail when the destination is blocked")
	}
	// The live node is untouched: still resolvable, without archive metadata.
	loc, _ := s.Locate("alpha/one/task-a")
	if loc == nil {
		t.Fatal("task should still be in the live tree after a failed archive")
	}
	if loc.Node.ArchivedRef != "" || loc.Node.ArchivedAt != "" {
		t.Errorf("archive metadata leaked onto the live node: {from:%q at:%q}",
			loc.Node.ArchivedRef, loc.Node.ArchivedAt)
	}
}

// Archiving an epic takes its whole subtree (the epic directory with its issues
// and their tasks); it is recovered as an epic and restores to its bare origin
// ref with every descendant riding along.
func TestArchiveEpicSubtreeRoundTrip(t *testing.T) {
	s := Open(fixture(t))
	epic, _ := s.Get("alpha")
	ref, err := s.Archive(epic, "2026-07-27T09:00:00Z")
	if err != nil {
		t.Fatalf("Archive epic: %v", err)
	}
	if ref != "_archives/alpha" {
		t.Errorf("archive ref = %q, want _archives/alpha", ref)
	}
	// Gone from the live tree, and the whole subtree moved under _archives/.
	if loc, _ := s.Locate("alpha"); loc != nil {
		t.Error("archived epic still resolves in the live tree")
	}
	for _, p := range []string{
		filepath.Join(ArchiveDir, "alpha", "_epic.md"),
		filepath.Join(ArchiveDir, "alpha", "one", "_issue.md"),
		filepath.Join(ArchiveDir, "alpha", "one", "task-a.md"),
	} {
		if _, err := os.Stat(filepath.Join(s.Root, p)); err != nil {
			t.Errorf("subtree member did not move with its epic: %s: %v", p, err)
		}
	}
	// Recovered as an epic, with its origin recorded.
	ae, err := s.ArchiveGet("_archives/alpha")
	if err != nil {
		t.Fatalf("ArchiveGet: %v", err)
	}
	if ae.Node.Type != model.Epic || ae.Node.ArchivedRef != "alpha" {
		t.Errorf("archived epic = {type:%q from:%q}", ae.Node.Type, ae.Node.ArchivedRef)
	}

	// Restores to its bare origin ref, descendants intact.
	restored, err := s.Unarchive(ae, "", "2026-07-28")
	if err != nil {
		t.Fatalf("Unarchive epic: %v", err)
	}
	if restored != "alpha" {
		t.Errorf("restored ref = %q, want alpha", restored)
	}
	for _, r := range []string{"alpha", "alpha/one", "alpha/one/task-a", "alpha/two"} {
		if loc, _ := s.Locate(r); loc == nil {
			t.Errorf("descendant %q not restored to the live tree", r)
		}
	}
	// Archive metadata is cleared on the restored epic, and it left the archive.
	loc, _ := s.Locate("alpha")
	if loc.Node.ArchivedRef != "" || loc.Node.ArchivedAt != "" {
		t.Errorf("archive metadata not cleared: %+v", loc.Node)
	}
	if got, _ := s.ArchiveLocate("_archives/alpha"); got != nil {
		t.Error("archived entry should be gone after restore")
	}
}

// Unarchive with no destination restores the node to its recorded origin.
func TestUnarchiveToOrigin(t *testing.T) {
	s := Open(fixture(t))
	task, _ := s.Get("alpha/one/task-a")
	if _, err := s.Archive(task, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	ae, _ := s.ArchiveGet("_archives/task-a")
	ref, err := s.Unarchive(ae, "", "2026-07-28")
	if err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	if ref != "alpha/one/task-a" {
		t.Errorf("restored ref = %q, want alpha/one/task-a", ref)
	}
	loc, _ := s.Locate("alpha/one/task-a")
	if loc == nil {
		t.Fatal("task not restored to its origin")
	}
	// Archive metadata is cleared on restore.
	if loc.Node.ArchivedRef != "" || loc.Node.ArchivedAt != "" {
		t.Errorf("archive metadata not cleared: %+v", loc.Node)
	}
	// And it is gone from the archive.
	if got, _ := s.ArchiveLocate("_archives/task-a"); got != nil {
		t.Error("archived entry should be gone after restore")
	}
}

// Restoring onto an origin whose slot is now occupied is an error, not an
// overwrite.
func TestUnarchiveCollisionErrors(t *testing.T) {
	s := Open(fixture(t))
	task, _ := s.Get("alpha/one/task-a")
	if _, err := s.Archive(task, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// Recreate a node at the same origin ref while the original is archived.
	write(t, filepath.Join(s.Root, "alpha", "one", "task-a.md"), "---\ntitle: New A\n---\n")
	ae, _ := s.ArchiveGet("_archives/task-a")
	if _, err := s.Unarchive(ae, "", "2026-07-28"); err == nil {
		t.Error("expected a collision error restoring onto an occupied ref")
	}
	// The occupying node is untouched (no overwrite).
	loc, _ := s.Locate("alpha/one/task-a")
	if loc == nil || loc.Node.Title != "New A" {
		t.Errorf("occupying node was overwritten: %+v", loc)
	}
}

// --to restores to an explicit destination instead of the recorded origin.
func TestUnarchiveTo(t *testing.T) {
	s := Open(fixture(t))
	task, _ := s.Get("alpha/one/task-a")
	if _, err := s.Archive(task, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	ae, _ := s.ArchiveGet("_archives/task-a")
	ref, err := s.Unarchive(ae, "alpha/two/moved", "2026-07-28")
	if err != nil {
		t.Fatalf("Unarchive --to: %v", err)
	}
	if ref != "alpha/two/moved" {
		t.Errorf("restored ref = %q, want alpha/two/moved", ref)
	}
	if loc, _ := s.Locate("alpha/two/moved"); loc == nil {
		t.Error("task not restored to the --to destination")
	}
	if loc, _ := s.Locate("alpha/one/task-a"); loc != nil {
		t.Error("task should not be at its origin after --to restore")
	}
}

// Restoring when the origin's parent no longer exists is an error (use --to).
func TestUnarchiveParentGoneErrors(t *testing.T) {
	s := Open(fixture(t))
	task, _ := s.Get("alpha/one/task-a")
	if _, err := s.Archive(task, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// Remove the origin's parent issue entirely.
	issue, _ := s.Get("alpha/one")
	if err := s.Remove(issue); err != nil {
		t.Fatal(err)
	}
	ae, _ := s.ArchiveGet("_archives/task-a")
	if _, err := s.Unarchive(ae, "", "2026-07-28"); err == nil {
		t.Error("expected an error: origin parent alpha/one no longer exists")
	}
}

// A second archive of a node sharing a slug gets a unique name under _archives/.
func TestArchiveNameCollision(t *testing.T) {
	s := Open(fixture(t))
	// alpha/one/task-a and a same-slug task under alpha/two.
	write(t, filepath.Join(s.Root, "alpha", "two", "task-a.md"), "---\ntitle: Other A\n---\n")

	a, _ := s.Get("alpha/one/task-a")
	r1, err := s.Archive(a, "2026-07-27T09:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get("alpha/two/task-a")
	r2, err := s.Archive(b, "2026-07-27T09:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if r1 == r2 {
		t.Errorf("second archive collided on name: %q == %q", r1, r2)
	}
	// Both are addressable and retain distinct origins.
	l := mustArchiveList(t, s)
	if len(l) != 2 {
		t.Fatalf("archive list count = %d, want 2", len(l))
	}
}

// Restoring to the origin leaves backlinks intact; restoring elsewhere rewrites
// [[origin]] to the new ref, like mv.
func TestUnarchiveBacklinks(t *testing.T) {
	s := mvFixture(t) // _orphan/ref links to [[alpha/one]] and [[alpha/one/t]]
	issue, _ := s.Get("alpha/one")
	if _, err := s.Archive(issue, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	ae, _ := s.ArchiveGet("_archives/one")
	if _, err := s.Unarchive(ae, "beta/one", "2026-07-28"); err != nil {
		t.Fatalf("Unarchive --to: %v", err)
	}
	ref, _ := s.Locate("_orphan/ref")
	if ref == nil {
		t.Fatal("ref issue missing")
	}
	if !strings.Contains(ref.Node.Body, "[[beta/one]]") {
		t.Errorf("origin backlink not rewritten to new ref: %q", ref.Node.Body)
	}
	if !strings.Contains(ref.Node.Body, "[[beta/one/t]]") {
		t.Errorf("descendant backlink not rewritten: %q", ref.Node.Body)
	}
}

// Restoring elsewhere with --to must not hijack backlinks that now belong to a
// different node reusing the origin ref: when the origin is reoccupied in the
// live tree, [[origin]] references are left pointing at the new occupant.
func TestUnarchiveToDoesNotHijackReusedOriginBacklinks(t *testing.T) {
	s := mvFixture(t) // _orphan/ref links to [[alpha/one]] and [[alpha/one/t]]
	issue, _ := s.Get("alpha/one")
	if _, err := s.Archive(issue, "2026-07-27T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// Recreate a live node at the origin ref while the original is archived; the
	// [[alpha/one]] references legitimately belong to this new node now.
	write(t, filepath.Join(s.Root, "alpha", "one", "_issue.md"), "---\ntitle: New One\n---\n")

	ae, _ := s.ArchiveGet("_archives/one")
	if _, err := s.Unarchive(ae, "beta/one", "2026-07-28"); err != nil {
		t.Fatalf("Unarchive --to: %v", err)
	}
	ref, _ := s.Locate("_orphan/ref")
	if ref == nil {
		t.Fatal("ref issue missing")
	}
	if strings.Contains(ref.Node.Body, "[[beta/one]]") {
		t.Errorf("backlink hijacked to the restored node: %q", ref.Node.Body)
	}
	if !strings.Contains(ref.Node.Body, "[[alpha/one]]") {
		t.Errorf("origin backlink should stay with the reused ref: %q", ref.Node.Body)
	}
}

func mustArchiveList(t *testing.T, s *Store) []*ArchiveEntry {
	t.Helper()
	l, err := s.ArchiveList()
	if err != nil {
		t.Fatalf("ArchiveList: %v", err)
	}
	return l
}
