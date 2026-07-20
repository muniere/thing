// Package store resolves the data directory and loads the tree from disk.
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

// DataDir resolves the directory that holds the node tree, and ConfigDir the
// one that holds config.yaml. Both follow the same precedence:
//
//	flag  ->  env  ->  nearest .thing/ (skipped with -g)  ->  XDG default
//
// The XDG defaults are $XDG_DATA_HOME/thing (else ~/.local/share/thing) for data
// and $XDG_CONFIG_HOME/thing (else ~/.config/thing) for config. A found project
// .thing/ holds both, so DataDir and ConfigDir agree there. Returned paths are
// not guaranteed to exist (init creates them).
func DataDir(flag string, global bool) (string, error) {
	return resolveDir(flag, global, "THING_DATA_DIR", "XDG_DATA_HOME", ".local/share/thing")
}

// ConfigDir resolves the directory that holds config.yaml. See DataDir.
func ConfigDir(flag string, global bool) (string, error) {
	return resolveDir(flag, global, "THING_CONFIG_DIR", "XDG_CONFIG_HOME", ".config/thing")
}

func resolveDir(flag string, global bool, env, xdgVar, homeRel string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if v := os.Getenv(env); v != "" {
		return v, nil
	}
	if !global {
		if found, ok := searchUp(); ok {
			return found, nil
		}
	}
	if xdg := os.Getenv(xdgVar); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "thing"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve dir: %w", err)
	}
	return filepath.Join(home, filepath.FromSlash(homeRel)), nil
}

// searchUp walks up from the working directory looking for a .thing directory,
// git-style.
func searchUp() (string, bool) {
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
// Epic statuses are rolled up from their issues unless set explicitly.
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

	if n.Status == "" {
		n.Status = rollupStatus(issues)
	}
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
	if n.Status == "" {
		n.Status = model.Todo
	}

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
		if n.Status == "" {
			n.Status = model.Todo
		}
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

// rollupStatus derives an epic's status from its issues:
// all done -> done; any doing -> doing; all todo -> todo; otherwise doing.
func rollupStatus(issues []*model.Node) model.Status {
	if len(issues) == 0 {
		return model.Todo
	}
	allDone, allTodo, anyDoing := true, true, false
	for _, is := range issues {
		st := is.Status
		if st == "" {
			st = model.Todo
		}
		if st != model.Done {
			allDone = false
		}
		if st != model.Todo {
			allTodo = false
		}
		if st == model.Doing {
			anyDoing = true
		}
	}
	switch {
	case allDone:
		return model.Done
	case anyDoing:
		return model.Doing
	case allTodo:
		return model.Todo
	default:
		return model.Doing
	}
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
