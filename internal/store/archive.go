package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

// ArchiveEntry pairs an archived node with its place under _archive/. The Node
// is the top of an archived subtree, with ArchivedRef/ArchivedAt set. Name is
// the on-disk name (machine-generated, unique within _archive/) and Ref is
// "_archive/<name>". File is the node's own Markdown file; Dir is the directory
// it owns (the _archive/ dir itself for a task, since a task is a single file).
type ArchiveEntry struct {
	Node *model.Node
	Ref  string
	Name string
	File string
	Dir  string
}

// archiveDir is the OS path of this store's _archive/ directory.
func (s *Store) archiveDir() string { return filepath.Join(s.Root, ArchiveDir) }

// archiveNames returns the entry names already used under _archive/ (directory
// names and file basenames without the ".md"), the uniqueness scope for a newly
// archived node's on-disk name.
func (s *Store) archiveNames() (map[string]bool, error) {
	set := make(map[string]bool)
	entries, err := os.ReadDir(s.archiveDir())
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			set[name] = true
		} else if strings.HasSuffix(name, ".md") {
			set[strings.TrimSuffix(name, ".md")] = true
		}
	}
	return set, nil
}

// Archive moves the node at e out of the live tree and into _archive/, recording
// its source ref in the node's frontmatter so it can be restored later. An epic
// or issue takes its whole subtree (its directory); a task takes only its file.
// The on-disk name under _archive/ is made unique, so the archived node's own
// slug is irrelevant to callers — its identity while archived is "_archive/<name>",
// and its source lives in ArchivedRef. at is the RFC3339 instant it is archived:
// its full value is recorded as archived_at and its date stamps Updated. Backlinks
// are left as-is (dangling while archived, like Remove), and re-resolve if the node
// is restored to its source.
func (s *Store) Archive(e *Entry, at string) (string, error) {
	names, err := s.archiveNames()
	if err != nil {
		return "", err
	}
	name := slug.Unique(e.Node.Slug, names)

	if err := os.MkdirAll(s.archiveDir(), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(s.archiveDir(), name)
	src := e.Dir
	if e.Node.Type == model.Task {
		dst += ".md"
		src = e.File
	}
	// Move first, then stamp the archive metadata at the destination, so a failed
	// relocation leaves the still-live node untouched and the archive retryable.
	if err := os.Rename(src, dst); err != nil {
		return "", fmt.Errorf("archive: relocate %s: %w", src, err)
	}
	marker := dst
	if e.Node.Type != model.Task {
		marker = filepath.Join(dst, markerFile(e.Node.Type))
	}
	e.Node.ArchivedRef = e.Ref
	e.Node.ArchivedAt = at
	e.Node.Updated = at[:len("2006-01-02")] // date portion of the RFC3339 instant
	if err := writeNode(marker, e.Node); err != nil {
		return "", fmt.Errorf("archive: save %s: %w", marker, err)
	}
	return ArchiveDir + "/" + name, nil
}

// ArchiveList reads every archived entry under _archive/. Each entry's type is
// recovered from its on-disk shape: a bare "*.md" file was a task, a directory
// with an _epic.md/_issue.md marker was an epic/issue. Children are not loaded —
// they travel with the directory on restore and are not needed in memory.
func (s *Store) ArchiveList() ([]*ArchiveEntry, error) {
	dir := s.archiveDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*ArchiveEntry
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			ae, err := loadArchiveDir(filepath.Join(dir, name), name)
			if err != nil {
				return nil, err
			}
			if ae != nil {
				out = append(out, ae)
			}
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		base := strings.TrimSuffix(name, ".md")
		n, err := loadNodeFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		n.Type = model.Task
		n.Slug = base
		out = append(out, &ArchiveEntry{Node: n, Ref: ArchiveDir + "/" + base, Name: base, File: filepath.Join(dir, name), Dir: dir})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// loadArchiveDir loads a directory entry under _archive/ as an epic or issue,
// deciding the type from which marker file it holds. A directory with neither
// marker is not a node and yields nil (skipped).
func loadArchiveDir(path, name string) (*ArchiveEntry, error) {
	var typ model.NodeType
	switch {
	case fileExists(filepath.Join(path, epicFile)):
		typ = model.Epic
	case fileExists(filepath.Join(path, issueFile)):
		typ = model.Issue
	default:
		return nil, nil
	}
	n, err := loadNodeFile(filepath.Join(path, markerFile(typ)))
	if err != nil {
		return nil, err
	}
	n.Type = typ
	n.Slug = name
	return &ArchiveEntry{Node: n, Ref: ArchiveDir + "/" + name, Name: name, File: filepath.Join(path, markerFile(typ)), Dir: path}, nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// ArchiveLocate returns the archived entry at ref ("_archive/<name>"), or nil.
func (s *Store) ArchiveLocate(ref string) (*ArchiveEntry, error) {
	list, err := s.ArchiveList()
	if err != nil {
		return nil, err
	}
	for _, ae := range list {
		if ae.Ref == ref {
			return ae, nil
		}
	}
	return nil, nil
}

// ArchiveGet is ArchiveLocate that errors when nothing is at ref.
func (s *Store) ArchiveGet(ref string) (*ArchiveEntry, error) {
	ae, err := s.ArchiveLocate(ref)
	if err != nil {
		return nil, err
	}
	if ae == nil {
		return nil, fmt.Errorf("no such archived node %q", ref)
	}
	return ae, nil
}

// Unarchive restores an archived entry to the live tree. The destination is the
// node's recorded source (ArchivedRef) unless to is non-empty, which overrides
// it. It validates the destination's parent exists and is a legal container, and
// that the leaf slug is free among its siblings — a collision or a missing parent
// is an error (the caller can retry with an explicit destination); an existing
// node is never overwritten. On success it clears the archive metadata, stamps
// updated, moves the file/subtree back, and — when the restore lands somewhere
// other than the source — rewrites "[[source]]" backlinks (and descendants') to
// the new ref, mirroring Mv. It returns the restored ref.
func (s *Store) Unarchive(ae *ArchiveEntry, to, updated string) (string, error) {
	source := ae.Node.ArchivedRef
	target := source
	if to != "" {
		target = to
	}
	if target == "" {
		return "", fmt.Errorf("unarchive: no recorded source; specify a destination")
	}
	parent, leaf := splitRef(target)
	if leaf == "" {
		return "", fmt.Errorf("unarchive: a destination is required")
	}
	container, err := s.destContainer(ae.Node.Type, parent)
	if err != nil {
		return "", err
	}
	newSlug := slug.Slugify(leaf)
	sib, err := s.siblingSlugs(parent)
	if err != nil {
		return "", err
	}
	if sib[newSlug] {
		return "", fmt.Errorf("%q already exists under %q", newSlug, parent)
	}

	// Clear the archive metadata before the move; relocate's Save persists it.
	ae.Node.ArchivedRef = ""
	ae.Node.ArchivedAt = ""

	// Restoring is a move back into the live tree, so reuse the same relocate the
	// live Mv uses: it renames the file/subtree into container under newSlug and
	// re-saves. Build a live Entry from the archived one for it to operate on.
	e := &Entry{Node: ae.Node, Ref: ae.Ref, File: ae.File, Dir: ae.Dir, Parent: parent}
	if err := s.relocate(e, container, newSlug, updated); err != nil {
		return "", fmt.Errorf("unarchive: %w", err)
	}

	newRef := newSlug
	if parent != "" {
		newRef = parent + "/" + newSlug
	}
	// Rewrite [[source]] backlinks to the new ref, mirroring Mv — but only while the
	// source ref is still free. If another node reused the source ref while this one
	// was archived, its [[source]] references belong to that occupant, so leave them
	// (the restored node's own backlinks then stay dangling; see README).
	if source != "" && newRef != source {
		occupied, err := s.Locate(source)
		if err != nil {
			return "", err
		}
		if occupied == nil {
			if err := s.rewriteBacklinks(source, newRef); err != nil {
				return "", fmt.Errorf("unarchive: node restored, but backlinks were only partly rewritten: %w", err)
			}
		}
	}
	return newRef, nil
}
