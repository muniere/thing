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

// Located pins a loaded node to its place on disk.
type Located struct {
	Node   *model.Node // the loaded node
	File   string      // path to the node's Markdown file
	Dir    string      // directory owned by the node (issue dir for a task)
	Parent string      // parent slug ("" for an epic or an orphan issue)
}

// Index loads the tree and returns every node keyed by slug, each paired with
// its on-disk location. Slugs are globally unique, so a flat index is enough to
// resolve any ref.
func (s *Store) Index() (map[string]*Located, error) {
	top, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := make(map[string]*Located)
	for _, n := range top {
		if n.Type == model.Epic {
			dir := filepath.Join(s.Root, n.Slug)
			idx[n.Slug] = &Located{Node: n, File: filepath.Join(dir, epicFile), Dir: dir}
			for _, issue := range n.Children {
				indexIssue(idx, issue, dir, n.Slug)
			}
		} else { // orphan issue
			dir := filepath.Join(s.Root, OrphanDir, n.Slug)
			idx[n.Slug] = &Located{Node: n, File: filepath.Join(dir, issueFile), Dir: dir}
			for _, task := range n.Children {
				idx[task.Slug] = &Located{Node: task, File: filepath.Join(dir, task.Slug+".md"), Dir: dir, Parent: n.Slug}
			}
		}
	}
	return idx, nil
}

func indexIssue(idx map[string]*Located, issue *model.Node, epicDir, epicSlug string) {
	dir := filepath.Join(epicDir, issue.Slug)
	idx[issue.Slug] = &Located{Node: issue, File: filepath.Join(dir, issueFile), Dir: dir, Parent: epicSlug}
	for _, task := range issue.Children {
		idx[task.Slug] = &Located{Node: task, File: filepath.Join(dir, task.Slug+".md"), Dir: dir, Parent: issue.Slug}
	}
}

// Locate returns the node with the given slug, or nil if it does not exist.
func (s *Store) Locate(slug string) (*Located, error) {
	idx, err := s.Index()
	if err != nil {
		return nil, err
	}
	return idx[slug], nil
}

// Find looks up slug and requires it to be of the given type, erroring if it is
// missing or of another type.
func (s *Store) Find(slug string, typ model.NodeType) (*Located, error) {
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
func (s *Store) Save(loc *Located) error {
	return writeNode(loc.File, loc.Node)
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
