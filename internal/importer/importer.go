// Package importer bulk-creates nodes from a JSON batch.
package importer

import (
	"encoding/json"
	"fmt"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
	"github.com/muniere/thing/internal/store"
)

// InboxSlug is the slug of the auto-created orphan issue that receives tasks
// whose parent is the special value "inbox".
const InboxSlug = "inbox"

// ResultStatus is the outcome of importing one item.
type ResultStatus string

const (
	// StatusCreated means the node was written.
	StatusCreated ResultStatus = "created"
	// StatusValidated means a dry-run accepted the item and assigned it a ref
	// without writing anything. It is not a promise that a real import would
	// succeed: dry-run cannot see write-time (IO/serialization) failures.
	StatusValidated ResultStatus = "validated"
	// StatusError means the item was rejected; Message says why.
	StatusError ResultStatus = "error"
)

// Item is one node in an import batch. Only Title is required; Type defaults to
// "task". Parent is a ref (slug-path) to the parent node, or the special value
// "inbox" for a task, or empty (or "_orphan") for an epic / orphan issue. The
// batch is a flat list: unlike an exported tree it has no children, so an
// export file is not an import file.
type Item struct {
	Type     string       `json:"type,omitempty"`
	Title    string       `json:"title"`
	Parent   string       `json:"parent,omitempty"`
	Priority string       `json:"priority,omitempty"`
	Category string       `json:"category,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Links    []model.Link `json:"links,omitempty"`
	Body     string       `json:"body,omitempty"`
}

// Result reports the outcome for one input item, in input order.
type Result struct {
	Title string `json:"title"`
	Ref   string `json:"ref,omitempty"`
	// Parent is the resolved add-target ref for a created/validated item, or the
	// raw input parent for an item that errors before its parent is resolved.
	Parent  string       `json:"parent,omitempty"`
	Status  ResultStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// Run parses the batch and creates each node. Malformed JSON is a hard error; a
// per-item failure yields a Result with status "error" without stopping the
// rest. The bool return is true when every item succeeded. With dryRun set,
// refs are assigned and parents validated but nothing is written.
func Run(s *store.Store, data []byte, dryRun bool, updated string) ([]Result, bool, error) {
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, fmt.Errorf("invalid import JSON: %w", err)
	}

	// refType and siblings mirror the tree — pre-existing plus created this
	// batch — so items can reference parents created earlier in the same batch,
	// and dry-run can predict slugs without writing.
	idx, err := s.Index()
	if err != nil {
		return nil, false, err
	}
	refType := make(map[string]model.NodeType, len(idx))
	siblings := make(map[string]map[string]bool)
	for _, e := range idx {
		refType[e.Ref] = e.Node.Type
		mark(siblings, e.Parent, e.Node.Slug)
	}

	ctx := &batch{s: s, refType: refType, siblings: siblings, dryRun: dryRun, updated: updated}
	results := make([]Result, 0, len(items))
	allOK := true
	for _, it := range items {
		r := ctx.one(it)
		if r.Status == StatusError {
			allOK = false
		}
		results = append(results, r)
	}
	return results, allOK, nil
}

type batch struct {
	s        *store.Store
	refType  map[string]model.NodeType
	siblings map[string]map[string]bool
	dryRun   bool
	updated  string
}

func (b *batch) one(it Item) Result {
	r := Result{Title: it.Title, Parent: it.Parent}

	if it.Title == "" {
		return errResult(r, "title is required")
	}
	typ := model.Task
	if it.Type != "" {
		typ = model.NodeType(it.Type)
	}
	if !typ.Valid() {
		return errResult(r, fmt.Sprintf("invalid type %q", typ))
	}
	prio := model.Priority(it.Priority)
	if prio != "" && !prio.Valid() {
		return errResult(r, fmt.Sprintf("invalid priority %q", prio))
	}
	if it.Category != "" && typ != model.Epic {
		return errResult(r, "category applies only to an epic")
	}
	for _, l := range it.Links {
		if l.URL == "" {
			return errResult(r, "link url is required")
		}
	}

	addRef, err := b.resolveParent(typ, it.Parent)
	if err != nil {
		return errResult(r, err.Error())
	}
	r.Parent = addRef

	n := &model.Node{
		Title:    it.Title,
		Category: it.Category,
		Priority: prio,
		Tags:     it.Tags,
		Updated:  b.updated,
		Links:    it.Links,
		Body:     it.Body,
	}
	ref, err := b.add(addRef, typ, n)
	if err != nil {
		return errResult(r, err.Error())
	}
	r.Ref = ref
	if b.dryRun {
		r.Status = StatusValidated
	} else {
		r.Status = StatusCreated
	}
	return r
}

// resolveParent maps the declared type and parent ref to the ref to add under,
// validating that the parent exists and is the expected kind.
func (b *batch) resolveParent(typ model.NodeType, parent string) (string, error) {
	switch typ {
	case model.Epic:
		if parent != "" {
			return "", fmt.Errorf("an epic takes no parent (got %q)", parent)
		}
		return "", nil
	case model.Issue:
		if parent == "" || parent == store.OrphanDir {
			return store.OrphanDir, nil // orphan
		}
		if b.refType[parent] != model.Epic {
			return "", fmt.Errorf("no such epic %q", parent)
		}
		return parent, nil
	case model.Task:
		if parent == "" {
			return "", fmt.Errorf("a task requires a parent issue")
		}
		if parent == InboxSlug {
			return b.ensureInbox()
		}
		if b.refType[parent] != model.Issue {
			return "", fmt.Errorf("no such issue %q", parent)
		}
		return parent, nil
	}
	return "", fmt.Errorf("invalid type %q", typ)
}

// add writes n under addRef (or simulates it under dry-run), returning the new
// ref and recording it so later items in the batch can see it.
func (b *batch) add(addRef string, typ model.NodeType, n *model.Node) (string, error) {
	if b.dryRun {
		n.Type = typ
		n.Slug = slug.Unique(slug.Slugify(n.Title), b.siblings[addRef])
		ref := joinRef(addRef, n.Slug)
		b.record(addRef, ref, n.Slug, typ)
		return ref, nil
	}
	ref, err := b.s.Add(addRef, n)
	if err != nil {
		return "", err
	}
	b.record(addRef, ref, n.Slug, typ)
	return ref, nil
}

// ensureInbox creates (or reuses) the orphan "inbox" issue and returns its ref.
func (b *batch) ensureInbox() (string, error) {
	ref := joinRef(store.OrphanDir, InboxSlug)
	if t, ok := b.refType[ref]; ok {
		if t != model.Issue {
			return "", fmt.Errorf("%q is not an issue", ref)
		}
		return ref, nil
	}
	n := &model.Node{Title: "Inbox", Updated: b.updated}
	return b.add(store.OrphanDir, model.Issue, n)
}

// record marks a freshly added node in the in-memory tree mirror.
func (b *batch) record(parentRef, ref, slug string, typ model.NodeType) {
	b.refType[ref] = typ
	mark(b.siblings, parentRef, slug)
}

func joinRef(parentRef, slug string) string {
	if parentRef == "" {
		return slug
	}
	return parentRef + "/" + slug
}

func mark(m map[string]map[string]bool, parentRef, slug string) {
	if m[parentRef] == nil {
		m[parentRef] = make(map[string]bool)
	}
	m[parentRef][slug] = true
}

func errResult(r Result, msg string) Result {
	r.Status = StatusError
	r.Message = msg
	return r
}
