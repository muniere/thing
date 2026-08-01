package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// archive moves a node out of the live tree; ls _archive lists it with its
// origin; show reaches it by its archive ref; unarchive restores it.
func TestArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)

	// archive prints the archive ref.
	code, out, errb := runCLI(t, append([]string{"archive", "web/roll/do"}, d...)...)
	if code != 0 {
		t.Fatalf("archive: code=%d err=%s", code, errb)
	}
	if strings.TrimSpace(out) != "_archive/do" {
		t.Fatalf("archive out = %q, want _archive/do", out)
	}

	// Gone from the live tree.
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll/do"}, d...)...); code == 0 {
		t.Error("archived task should not resolve in the live tree")
	}
	if _, out, _ := runCLI(t, append([]string{"tree"}, d...)...); strings.Contains(out, "(do)") {
		t.Errorf("archived task should not appear in tree: %q", out)
	}

	// ls --archived lists the entry with its origin ref.
	_, out, _ = runCLI(t, append([]string{"ls", "--archived"}, d...)...)
	if !strings.Contains(out, "_archive/do") || !strings.Contains(out, "web/roll/do") {
		t.Errorf("ls --archived: %q", out)
	}

	// show reaches the archived node by its archive ref.
	if _, out, _ := runCLI(t, append([]string{"show", "_archive/do"}, d...)...); !strings.Contains(out, "Do") {
		t.Errorf("show _archive/do: %q", out)
	}

	// unarchive restores it to its origin and prints the restored ref.
	if _, out, _ := runCLI(t, append([]string{"unarchive", "_archive/do"}, d...)...); strings.TrimSpace(out) != "web/roll/do" {
		t.Errorf("unarchive out = %q, want web/roll/do", out)
	}
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll/do"}, d...)...); code != 0 {
		t.Error("task should be restored to the live tree")
	}
}

// archive stamps archived_at as an RFC3339 instant that includes a time-of-day,
// not just a date — the CLI records the moment of archiving.
func TestArchiveStampsTimestamp(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)
	runCLI(t, append([]string{"archive", "web/roll/do"}, d...)...)

	data, err := os.ReadFile(filepath.Join(dir, "_archive", "do.md"))
	if err != nil {
		t.Fatalf("read archived file: %v", err)
	}
	var line string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "archived_at:") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("archived_at not written:\n%s", data)
	}
	// RFC3339 separates the date and time with a "T"; a date-only stamp has none.
	if !strings.Contains(line, "T") {
		t.Errorf("archived_at is not an RFC3339 timestamp with a time: %q", line)
	}
}

// ls --archived lists only archived entries; ls --all lists the live top level
// plus the archived entries; the old "_archive" pseudo-ref no longer resolves.
func TestLsArchivedAndAll(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)
	runCLI(t, append([]string{"archive", "web/roll/do"}, d...)...)

	// --archived: only the archived entry, not the live epic.
	code, out, errb := runCLI(t, append([]string{"ls", "--archived"}, d...)...)
	if code != 0 {
		t.Fatalf("ls --archived: code=%d err=%s", code, errb)
	}
	if !strings.Contains(out, "_archive/do") || !strings.Contains(out, "web/roll/do") {
		t.Errorf("ls --archived missing the archived entry: %q", out)
	}
	if strings.Contains(out, "Web") {
		t.Errorf("ls --archived should not list live epics: %q", out)
	}

	// --all: the live top level AND the archived entry.
	if _, out, _ := runCLI(t, append([]string{"ls", "--all"}, d...)...); !strings.Contains(out, "Web") || !strings.Contains(out, "_archive/do") {
		t.Errorf("ls --all should list both live and archived: %q", out)
	}

	// The pseudo-ref is gone: "_archive" no longer resolves as a listable ref.
	if code, _, _ := runCLI(t, append([]string{"ls", "_archive"}, d...)...); code == 0 {
		t.Error("ls _archive should no longer resolve as a pseudo-ref")
	}

	// --archived and --all are mutually exclusive.
	if code, _, _ := runCLI(t, append([]string{"ls", "--archived", "--all"}, d...)...); code == 0 {
		t.Error("ls --archived --all should be rejected as mutually exclusive")
	}
}

// unarchive onto an occupied origin errors; --to restores elsewhere.
func TestUnarchiveToAndCollision(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}
	runCLI(t, append([]string{"init"}, d...)...)
	runCLI(t, append([]string{"add", "Web"}, d...)...)
	runCLI(t, append([]string{"add", "web/Roll"}, d...)...)
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)
	runCLI(t, append([]string{"archive", "web/roll/do"}, d...)...)

	// Recreate a node at the origin while the original is archived.
	runCLI(t, append([]string{"add", "web/roll/Do"}, d...)...)

	// Restoring to the now-occupied origin fails.
	if code, _, _ := runCLI(t, append([]string{"unarchive", "_archive/do"}, d...)...); code == 0 {
		t.Error("unarchive onto an occupied origin should fail")
	}
	// --to restores to an explicit destination.
	if code, out, errb := runCLI(t, append([]string{"unarchive", "_archive/do", "--to", "web/roll/done"}, d...)...); code != 0 || strings.TrimSpace(out) != "web/roll/done" {
		t.Fatalf("unarchive --to: code=%d out=%q err=%s", code, out, errb)
	}
	if code, _, _ := runCLI(t, append([]string{"show", "web/roll/done"}, d...)...); code != 0 {
		t.Error("task should be restored at the --to destination")
	}

	// unarchive of an unknown archive ref errors.
	if code, _, _ := runCLI(t, append([]string{"unarchive", "_archive/ghost"}, d...)...); code == 0 {
		t.Error("unarchive of an unknown archive ref should fail")
	}
	// archive of an unknown ref errors.
	if code, _, _ := runCLI(t, append([]string{"archive", "nope"}, d...)...); code == 0 {
		t.Error("archive of an unknown ref should fail")
	}
}
