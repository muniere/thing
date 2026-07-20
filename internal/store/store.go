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
	"strings"

	"github.com/muniere/thing/internal/frontmatter"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

const (
	// OrphanDir holds issues that belong to no epic.
	OrphanDir = "_orphan"
	epicFile  = "_epic.md"
	issueFile = "_issue.md"
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
	File   string      // path to the node's Markdown file
	Dir    string      // directory owned by the node (issue dir for a task)
	Parent string      // parent slug ("" for an epic or an orphan issue)
}

// Index loads the tree and returns every node keyed by slug, each paired with
// its on-disk location. Slugs are globally unique, so a flat index is enough to
// resolve any ref.
func (s *Store) Index() (map[string]*Entry, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := make(map[string]*Entry)
	for _, n := range top {
		if n.Type == model.Epic {
			dir := filepath.Join(s.Root, n.Slug)
			idx[n.Slug] = &Entry{Node: n, File: filepath.Join(dir, epicFile), Dir: dir}
			for _, issue := range n.Children {
				indexIssue(idx, issue, dir, n.Slug)
			}
		} else { // orphan issue
			dir := filepath.Join(s.Root, OrphanDir, n.Slug)
			idx[n.Slug] = &Entry{Node: n, File: filepath.Join(dir, issueFile), Dir: dir}
			for _, task := range n.Children {
				idx[task.Slug] = &Entry{Node: task, File: filepath.Join(dir, task.Slug+".md"), Dir: dir, Parent: n.Slug}
			}
		}
	}
	return idx, nil
}

func indexIssue(idx map[string]*Entry, issue *model.Node, epicDir, epicSlug string) {
	dir := filepath.Join(epicDir, issue.Slug)
	idx[issue.Slug] = &Entry{Node: issue, File: filepath.Join(dir, issueFile), Dir: dir, Parent: epicSlug}
	for _, task := range issue.Children {
		idx[task.Slug] = &Entry{Node: task, File: filepath.Join(dir, task.Slug+".md"), Dir: dir, Parent: issue.Slug}
	}
}

// Locate returns the node with the given slug, or nil if it does not exist.
func (s *Store) Locate(slug string) (*Entry, error) {
	idx, err := s.Index()
	if err != nil {
		return nil, err
	}
	return idx[slug], nil
}

// Get looks up slug and errors if no node has it. Unlike Find it does not
// constrain the node's type — a slug is globally unique and carries its own.
func (s *Store) Get(slug string) (*Entry, error) {
	loc, err := s.Locate(slug)
	if err != nil {
		return nil, err
	}
	if loc == nil {
		return nil, fmt.Errorf("no such node %q", slug)
	}
	return loc, nil
}

// Find looks up slug and requires it to be of the given type, erroring if it is
// missing or of another type.
func (s *Store) Find(slug string, typ model.NodeType) (*Entry, error) {
	loc, err := s.Locate(slug)
	if err != nil {
		return nil, err
	}
	if loc == nil || loc.Node.Type != typ {
		return nil, fmt.Errorf("no such %s %q", typ, slug)
	}
	return loc, nil
}

// AllSlugs returns the set of every slug in the tree (the global uniqueness
// scope for new slugs).
func (s *Store) AllSlugs() (map[string]bool, error) {
	idx, err := s.Index()
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(idx))
	for slug := range idx {
		set[slug] = true
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

// CreateEpic writes a new epic node under the root.
func (s *Store) CreateEpic(n *model.Node) error {
	n.Type = model.Epic
	return writeNode(filepath.Join(s.Root, n.Slug, epicFile), n)
}

// CreateIssue writes a new issue node under the given epic, or under _orphan
// when epicSlug is empty.
func (s *Store) CreateIssue(n *model.Node, epicSlug string) error {
	n.Type = model.Issue
	var dir string
	if epicSlug == "" {
		dir = filepath.Join(s.Root, OrphanDir, n.Slug)
	} else {
		dir = filepath.Join(s.Root, epicSlug, n.Slug)
	}
	return writeNode(filepath.Join(dir, issueFile), n)
}

// CreateTask writes a new task node in the given issue directory.
func (s *Store) CreateTask(n *model.Node, issueDir string) error {
	n.Type = model.Task
	return writeNode(filepath.Join(issueDir, n.Slug+".md"), n)
}

// Save re-serializes a located node back to its own file. Callers set the
// node's fields (and its Updated date) before calling. Because Load leaves
// Status exactly as the file holds it, saving a node whose status the file
// omits does not write a derived status — only an explicitly set one persists.
func (s *Store) Save(loc *Entry) error {
	return writeNode(loc.File, loc.Node)
}

// Remove deletes a node. An epic or issue takes its whole subtree (its
// directory); a task takes only its file. Any "[[slug]]" backlinks to the
// removed node are left as-is (dangling).
func (s *Store) Remove(e *Entry) error {
	switch e.Node.Type {
	case model.Epic, model.Issue:
		return os.RemoveAll(e.Dir)
	default: // task
		return os.Remove(e.File)
	}
}

// Mv relocates the node named by src to dst, the way the shell's mv does. Each
// ref is a slug path "<parent>/<name>" — an epic is a bare "<name>", an orphan
// issue uses the "_orphan" parent. A changed parent moves the node; a changed
// name renames it, rewriting "[[old]]" backlinks across the tree to follow; a
// change to both does both. The name is the slug itself, not the title.
func (s *Store) Mv(src, dst, updated string) error {
	srcParent, srcName := splitRef(src)
	dstParent, dstName := splitRef(dst)
	if srcName == "" || dstName == "" {
		return fmt.Errorf("mv: a source and destination are required")
	}

	e, err := s.Locate(srcName)
	if err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("no such node %q", srcName)
	}
	if srcParent != parentRef(e) {
		return fmt.Errorf("%q is not at %q", srcName, src)
	}

	container, err := s.destContainer(e.Node.Type, dstParent)
	if err != nil {
		return err
	}

	newSlug := slug.Slugify(dstName)
	if newSlug != e.Node.Slug {
		taken, err := s.AllSlugs()
		if err != nil {
			return err
		}
		if taken[newSlug] {
			return fmt.Errorf("slug %q already exists", newSlug)
		}
	}
	return s.relocate(e, container, newSlug, updated)
}

// splitRef splits a "<parent>/<name>" ref into its parent and name; a ref with
// no slash has an empty parent.
func splitRef(ref string) (parent, name string) {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return "", ref
}

// parentRef is the path-parent an entry would appear under: "" for an epic,
// "_orphan" for an orphan issue, otherwise the parent slug.
func parentRef(e *Entry) string {
	switch e.Node.Type {
	case model.Epic:
		return ""
	case model.Issue:
		if e.Parent == "" {
			return OrphanDir
		}
		return e.Parent
	default: // task
		return e.Parent
	}
}

// destContainer resolves the directory that will hold a node of the given type
// under dstParent, validating that dstParent is a legal parent for that type.
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
// updated date, re-saves, and rewrites backlinks when the slug changed. The
// slug is the node's on-disk name, not a frontmatter field, so after the rename
// the node's identity already matches its location; a mid-way failure can leave
// a stale updated date or partly-rewritten backlinks, but not a broken slug.
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
	if newSlug != oldSlug {
		if err := s.rewriteBacklinks(oldSlug, newSlug); err != nil {
			return fmt.Errorf("mv: node moved, but backlinks were only partly rewritten: %w", err)
		}
	}
	return nil
}

// rewriteBacklinks replaces every "[[oldSlug]]" with "[[newSlug]]" across all
// node files. A file that vanished (e.g. removed concurrently) is skipped.
func (s *Store) rewriteBacklinks(oldSlug, newSlug string) error {
	idx, err := s.Index()
	if err != nil {
		return err
	}
	old, neu := "[["+oldSlug+"]]", "[["+newSlug+"]]"
	for _, e := range idx {
		data, err := os.ReadFile(e.File)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read %s: %w", e.File, err)
		}
		if !strings.Contains(string(data), old) {
			continue
		}
		if err := os.WriteFile(e.File, []byte(strings.ReplaceAll(string(data), old, neu)), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", e.File, err)
		}
	}
	return nil
}

func sortBySlug(nodes []*model.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Slug < nodes[j].Slug })
}

// Epics returns the top-level epics.
func (s *Store) Epics() ([]*model.Node, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	var out []*model.Node
	for _, n := range top {
		if n.Type == model.Epic {
			out = append(out, n)
		}
	}
	return out, nil
}

// Issues returns issues scoped to epicSlug, or every issue (including orphans)
// when epicSlug is empty.
func (s *Store) Issues(epicSlug string) ([]*model.Node, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	var out []*model.Node
	for _, n := range top {
		switch n.Type {
		case model.Epic:
			if epicSlug == "" || n.Slug == epicSlug {
				out = append(out, n.Children...)
			}
		case model.Issue: // orphan
			if epicSlug == "" {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// Tasks returns tasks scoped to issueSlug, or every task when issueSlug is empty.
func (s *Store) Tasks(issueSlug string) ([]*model.Node, error) {
	issues, err := s.Issues("")
	if err != nil {
		return nil, err
	}
	var out []*model.Node
	for _, is := range issues {
		if issueSlug == "" || is.Slug == issueSlug {
			out = append(out, is.Children...)
		}
	}
	return out, nil
}
