// Package store loads and mutates the node tree on disk. It also exposes the
// directory primitives — the global XDG directories and the upward .thing/
// search — that the CLI layer composes into a resolved data/config directory.
//
// On-disk layout:
//
//	<root>/config.yaml
//	<root>/<epic>/_epic.md
//	<root>/<epic>/<issue>/_issue.md
//	<root>/<epic>/<issue>/<task>.md
//	<root>/_orphan/<issue>/...        (issues that belong to no epic)
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/muniere/thing/internal/frontmatter"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

const (
	// OrphanDir holds issues that belong to no epic.
	OrphanDir = "_orphan"
	// ArchiveDir holds archived subtrees, hidden from the live tree. Its entries
	// are addressed as "_archives/<name>" and loaded only through the Archive* API.
	ArchiveDir = "_archives"
	epicFile   = "_epic.md"
	issueFile  = "_issue.md"
)

// ProjectDir is the per-project directory name, searched for upward git-style.
const ProjectDir = ".thing"

// GlobalDataDir is the global tree directory following the XDG spec:
// $XDG_DATA_HOME/thing when that is an absolute path, else ~/.local/share/thing.
func GlobalDataDir() (string, error) {
	return globalDir("XDG_DATA_HOME", ".local/share/thing")
}

// GlobalConfigDir is the global config directory following the XDG spec:
// $XDG_CONFIG_HOME/thing when that is an absolute path, else ~/.config/thing.
func GlobalConfigDir() (string, error) {
	return globalDir("XDG_CONFIG_HOME", ".config/thing")
}

// globalDir returns $<xdgVar>/thing when that XDG var is an absolute path,
// otherwise ~/<homeRel>.
func globalDir(xdgVar, homeRel string) (string, error) {
	if xdg := os.Getenv(xdgVar); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "thing"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve dir: %w", err)
	}
	return filepath.Join(home, filepath.FromSlash(homeRel)), nil
}

// FindProjectDir searches upward from the working directory for a project
// .thing/ directory, git-style, returning it if found.
func FindProjectDir() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		cand := filepath.Join(cwd, ".thing")
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand, true
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", false
		}
		cwd = parent
	}
}

// Store is a handle to a resolved data directory.
type Store struct {
	Root string
}

// Open returns a Store rooted at root.
func Open(root string) *Store {
	return &Store{Root: root}
}

// Load reads the entire tree. The returned slice holds the top-level nodes:
// epics (with their issues and tasks) and orphan issues, each sorted by slug.
// A node's Status is exactly what its file holds — empty when omitted; the
// display-time rollup and todo default live in model.Node.EffectiveStatus.
func (s *Store) Load() ([]*model.Node, error) {
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, err
	}

	var top []*model.Node
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == ArchiveDir {
			continue // archived subtrees are hidden from the live tree
		}
		if name == OrphanDir {
			issues, err := s.loadIssues(filepath.Join(s.Root, name))
			if err != nil {
				return nil, err
			}
			top = append(top, issues...)
			continue
		}
		epic, err := s.loadEpic(filepath.Join(s.Root, name), name)
		if err != nil {
			return nil, err
		}
		top = append(top, epic)
	}

	sortBySlug(top)
	return top, nil
}

func (s *Store) loadEpic(dir, slug string) (*model.Node, error) {
	n, err := loadNodeFile(filepath.Join(dir, epicFile))
	if err != nil {
		return nil, err
	}
	n.Type = model.Epic
	n.Slug = slug

	issues, err := s.loadIssues(dir)
	if err != nil {
		return nil, err
	}
	n.Children = issues
	return n, nil
}

// loadIssues reads every immediate subdirectory of dir as an issue.
func (s *Store) loadIssues(dir string) ([]*model.Node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var issues []*model.Node
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		issue, err := s.loadIssue(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	sortBySlug(issues)
	return issues, nil
}

func (s *Store) loadIssue(dir, slug string) (*model.Node, error) {
	n, err := loadNodeFile(filepath.Join(dir, issueFile))
	if err != nil {
		return nil, err
	}
	n.Type = model.Issue
	n.Slug = slug

	tasks, err := s.loadTasks(dir)
	if err != nil {
		return nil, err
	}
	n.Children = tasks
	return n, nil
}

// loadTasks reads every "*.md" file in dir (except _issue.md) as a task.
func (s *Store) loadTasks(dir string) ([]*model.Node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var tasks []*model.Node
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == issueFile || !strings.HasSuffix(name, ".md") {
			continue
		}
		n, err := loadNodeFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		n.Type = model.Task
		n.Slug = strings.TrimSuffix(name, ".md")
		tasks = append(tasks, n)
	}
	sortBySlug(tasks)
	return tasks, nil
}

// loadNodeFile parses a node file. A missing file yields an empty node so that
// a directory without its marker file still loads.
func loadNodeFile(path string) (*model.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.Node{}, nil
		}
		return nil, err
	}
	return frontmatter.Parse(data)
}

// Entry pairs a loaded node with its place on disk, as returned by Locate and
// stored in the Index.
type Entry struct {
	Node   *model.Node // the loaded node
	Ref    string      // the node's ref (identity): "epic", "epic/issue", "epic/issue/task", "_orphan/issue"
	File   string      // OS path to the node's Markdown file
	Dir    string      // OS directory owned by the node (issue dir for a task)
	Parent string      // the parent's ref ("" for a top-level epic, "_orphan" for an orphan issue)
}

// Index loads the tree and returns every node keyed by its ref. A slug is
// unique only among its siblings, so the full ref is the identity.
func (s *Store) Index() (map[string]*Entry, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := make(map[string]*Entry)
	for _, n := range top {
		if n.Type == model.Epic {
			s.indexNode(idx, s.Root, "", n)
		} else { // orphan issue
			s.indexNode(idx, filepath.Join(s.Root, OrphanDir), OrphanDir, n)
		}
	}
	return idx, nil
}

// indexNode records n and its descendants, deriving each node's ref from
// parentRef and its on-disk location from parentDir.
func (s *Store) indexNode(idx map[string]*Entry, parentDir, parentRef string, n *model.Node) {
	ref := n.Slug
	if parentRef != "" {
		ref = parentRef + "/" + n.Slug
	}
	if n.Type == model.Task {
		idx[ref] = &Entry{Node: n, Ref: ref, File: filepath.Join(parentDir, n.Slug+".md"), Dir: parentDir, Parent: parentRef}
		return
	}
	dir := filepath.Join(parentDir, n.Slug)
	idx[ref] = &Entry{Node: n, Ref: ref, File: filepath.Join(dir, markerFile(n.Type)), Dir: dir, Parent: parentRef}
	for _, c := range n.Children {
		s.indexNode(idx, dir, ref, c)
	}
}

// markerFile is the fixed file name that marks an epic or issue directory.
func markerFile(t model.NodeType) string {
	if t == model.Epic {
		return epicFile
	}
	return issueFile
}

// Locate returns the node at the given ref, or nil if none exists.
func (s *Store) Locate(ref string) (*Entry, error) {
	idx, err := s.Index()
	if err != nil {
		return nil, err
	}
	return idx[ref], nil
}

// Get looks up a ref and errors if no node is there. Unlike Find it does
// not constrain the node's type — a ref already identifies a single node.
func (s *Store) Get(ref string) (*Entry, error) {
	loc, err := s.Locate(ref)
	if err != nil {
		return nil, err
	}
	if loc == nil {
		return nil, fmt.Errorf("no such node %q", ref)
	}
	return loc, nil
}

// Find looks up a ref and requires the node to be of the given type,
// erroring if it is missing or of another type.
func (s *Store) Find(ref string, typ model.NodeType) (*Entry, error) {
	loc, err := s.Locate(ref)
	if err != nil {
		return nil, err
	}
	if loc == nil || loc.Node.Type != typ {
		return nil, fmt.Errorf("no such %s %q", typ, ref)
	}
	return loc, nil
}

// siblingSlugs returns the slugs already used directly under parentRef (an
// empty parentRef is the top level, i.e. epics). This is the uniqueness scope
// for a new slug: names collide only within one parent.
func (s *Store) siblingSlugs(parentRef string) (map[string]bool, error) {
	idx, err := s.Index()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	for _, e := range idx {
		if e.Parent == parentRef {
			set[e.Node.Slug] = true
		}
	}
	return set, nil
}

// writeNode marshals a node and writes it to path, creating parent dirs.
func writeNode(path string, n *model.Node) error {
	data, err := frontmatter.Marshal(n)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Add creates a node from n (with Title and any Priority/Tags/Category set)
// under parentRef — "" for a top-level epic, "_orphan" for an orphan issue,
// otherwise an epic ref (-> issue) or issue ref (-> task). The child's type
// follows its parent; its slug is made unique among its siblings. It returns
// the new node's ref.
func (s *Store) Add(parentRef string, n *model.Node) (string, error) {
	container, childType, err := s.addTarget(parentRef)
	if err != nil {
		return "", err
	}
	sib, err := s.siblingSlugs(parentRef)
	if err != nil {
		return "", err
	}
	n.Type = childType
	n.Slug = slug.Unique(slug.Slugify(n.Title), sib)

	file := filepath.Join(container, n.Slug+".md")
	if childType != model.Task {
		file = filepath.Join(container, n.Slug, markerFile(childType))
	}
	if err := writeNode(file, n); err != nil {
		return "", err
	}
	if parentRef == "" {
		return n.Slug, nil
	}
	return parentRef + "/" + n.Slug, nil
}

// addTarget resolves the directory that will hold a child of parentRef and the
// type such a child would be.
func (s *Store) addTarget(parentRef string) (container string, childType model.NodeType, err error) {
	switch parentRef {
	case "":
		return s.Root, model.Epic, nil
	case OrphanDir:
		return filepath.Join(s.Root, OrphanDir), model.Issue, nil
	}
	p, err := s.Get(parentRef)
	if err != nil {
		return "", "", err
	}
	switch p.Node.Type {
	case model.Epic:
		return p.Dir, model.Issue, nil
	case model.Issue:
		return p.Dir, model.Task, nil
	default:
		return "", "", fmt.Errorf("cannot add a node under a task")
	}
}

// Save re-serializes a located node back to its own file. Callers set the
// node's fields (and its Updated date) before calling. Because Load leaves
// Status exactly as the file holds it, saving a node whose status the file
// omits does not write a derived status — only an explicitly set one persists.
func (s *Store) Save(loc *Entry) error {
	return writeNode(loc.File, loc.Node)
}

// Remove deletes a node. An epic or issue takes its whole subtree (its
// directory); a task takes only its file. Any "[[ref]]" backlinks to the
// removed node are left as-is (dangling).
func (s *Store) Remove(e *Entry) error {
	switch e.Node.Type {
	case model.Epic, model.Issue:
		return os.RemoveAll(e.Dir)
	default: // task
		return os.Remove(e.File)
	}
}

// AddLink adds a related link to a node, or — when the URL is already present —
// overwrites its label (an empty label clears it), stamping the updated date.
func (s *Store) AddLink(e *Entry, url, label, updated string) error {
	for i := range e.Node.Links {
		if e.Node.Links[i].URL == url {
			e.Node.Links[i].Label = label
			e.Node.Updated = updated
			return s.Save(e)
		}
	}
	e.Node.Links = append(e.Node.Links, model.Link{URL: url, Label: label})
	e.Node.Updated = updated
	return s.Save(e)
}

// RemoveLink removes a link identified by its URL, or failing that by a 1-based
// index into the node's links, stamping the updated date.
func (s *Store) RemoveLink(e *Entry, which, updated string) error {
	links := e.Node.Links
	for i := range links {
		if links[i].URL == which {
			e.Node.Links = append(links[:i:i], links[i+1:]...)
			e.Node.Updated = updated
			return s.Save(e)
		}
	}
	if idx, err := strconv.Atoi(which); err == nil {
		if idx < 1 || idx > len(links) {
			return fmt.Errorf("link index %d out of range (1..%d)", idx, len(links))
		}
		e.Node.Links = append(links[:idx-1:idx-1], links[idx:]...)
		e.Node.Updated = updated
		return s.Save(e)
	}
	return fmt.Errorf("no link matching %q", which)
}

// Mv relocates the node at ref src to ref dst, the way the shell's mv does. dst
// is "<parent-ref>/<name>" — a bare "<name>" for an epic. A changed parent moves
// the node; a changed name renames it, rewriting "[[old-ref]]" backlinks (and
// those of its descendants) across the tree to follow; a change to both does
// both. The name is the slug, not the title.
func (s *Store) Mv(src, dst, updated string) error {
	e, err := s.Get(src)
	if err != nil {
		return err
	}
	dstParent, dstName := splitRef(dst)
	if dstName == "" {
		return fmt.Errorf("mv: a destination is required")
	}

	container, err := s.destContainer(e.Node.Type, dstParent)
	if err != nil {
		return err
	}

	newSlug := slug.Slugify(dstName)
	sib, err := s.siblingSlugs(dstParent)
	if err != nil {
		return err
	}
	if dstParent == e.Parent {
		delete(sib, e.Node.Slug) // the node does not collide with itself
	}
	if sib[newSlug] {
		return fmt.Errorf("%q already exists under %q", newSlug, dstParent)
	}

	oldRef := e.Ref
	if err := s.relocate(e, container, newSlug, updated); err != nil {
		return err
	}
	newRef := newSlug
	if dstParent != "" {
		newRef = dstParent + "/" + newSlug
	}
	if newRef != oldRef {
		if err := s.rewriteBacklinks(oldRef, newRef); err != nil {
			return fmt.Errorf("mv: node moved, but backlinks were only partly rewritten: %w", err)
		}
	}
	return nil
}

// splitRef splits a "<parent>/<name>" ref into its parent ref and name;
// a ref with no slash has an empty parent.
func splitRef(ref string) (parent, name string) {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return "", ref
}

// destContainer resolves the directory that will hold a node of the given type
// under dstParent (a ref), validating that dstParent is a legal parent.
func (s *Store) destContainer(typ model.NodeType, dstParent string) (string, error) {
	switch typ {
	case model.Epic:
		if dstParent != "" {
			return "", fmt.Errorf("an epic has no parent; destination must be a bare name")
		}
		return s.Root, nil
	case model.Issue:
		if dstParent == OrphanDir {
			return filepath.Join(s.Root, OrphanDir), nil
		}
		p, err := s.Find(dstParent, model.Epic)
		if err != nil {
			return "", err
		}
		return p.Dir, nil
	default: // task
		p, err := s.Find(dstParent, model.Issue)
		if err != nil {
			return "", err
		}
		return p.Dir, nil
	}
}

// relocate renames a node's file/dir into container under newSlug, stamps the
// updated date, and re-saves; a complete no-op (same parent and name) returns
// early without touching anything. The slug is the node's on-disk name, so after
// the rename its identity already matches its location. Rewriting backlinks is
// the caller's job (it needs the old and new refs).
func (s *Store) relocate(e *Entry, container, newSlug, updated string) error {
	oldSlug := e.Node.Slug
	moved := false
	switch e.Node.Type {
	case model.Epic, model.Issue:
		newDir := filepath.Join(container, newSlug)
		if newDir != e.Dir {
			if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
				return err
			}
			if err := os.Rename(e.Dir, newDir); err != nil {
				return fmt.Errorf("mv: relocate %s: %w", e.Dir, err)
			}
			e.File = filepath.Join(newDir, filepath.Base(e.File))
			e.Dir = newDir
			moved = true
		}
	default: // task
		newFile := filepath.Join(container, newSlug+".md")
		if newFile != e.File {
			if err := os.MkdirAll(container, 0o755); err != nil {
				return err
			}
			if err := os.Rename(e.File, newFile); err != nil {
				return fmt.Errorf("mv: relocate %s: %w", e.File, err)
			}
			e.File = newFile
			e.Dir = container
			moved = true
		}
	}
	// Nothing to do: same parent and same name.
	if !moved && newSlug == oldSlug {
		return nil
	}
	e.Node.Slug = newSlug
	e.Node.Updated = updated
	if err := s.Save(e); err != nil {
		return fmt.Errorf("mv: save %s: %w", e.File, err)
	}
	return nil
}

// rewriteBacklinks rewrites "[[oldRef]]" and any descendant "[[oldRef/...]]"
// references to use newRef, across every node file, so a moved or renamed
// node's links (and those to its descendants) follow. A file that vanished
// (e.g. removed concurrently) is skipped.
func (s *Store) rewriteBacklinks(oldRef, newRef string) error {
	idx, err := s.Index()
	if err != nil {
		return err
	}
	for _, e := range idx {
		data, err := os.ReadFile(e.File)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read %s: %w", e.File, err)
		}
		out := strings.ReplaceAll(string(data), "[["+oldRef+"]]", "[["+newRef+"]]")
		out = strings.ReplaceAll(out, "[["+oldRef+"/", "[["+newRef+"/")
		if out == string(data) {
			continue
		}
		if err := os.WriteFile(e.File, []byte(out), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.File, err)
		}
	}
	return nil
}

func sortBySlug(nodes []*model.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Slug < nodes[j].Slug })
}
