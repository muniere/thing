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

// Resolve determines the data directory using this precedence:
//
//	dir flag  ->  global (~/.thing)  ->  THING_DIR  ->  nearest .thing/  ->  ./.thing
//
// The returned path is not guaranteed to exist (init creates it).
func Resolve(dir string, global bool) (string, error) {
	if dir != "" {
		return dir, nil
	}
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve global dir: %w", err)
		}
		return filepath.Join(home, ".thing"), nil
	}
	if env := os.Getenv("THING_DIR"); env != "" {
		return env, nil
	}
	if found, ok := searchUp(); ok {
		return found, nil
	}
	return ".thing", nil
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
