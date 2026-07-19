package frontmatter

import (
	"strings"
	"testing"

	"github.com/muniere/thing/internal/model"
)

func TestParse(t *testing.T) {
	raw := `---
title: Monitor rollout
status: doing
priority: high
category: Project
tags:
    - release
    - ops
updated: "2026-07-19"
links:
    - url: https://example.com
      label: Design doc
---
This is the **body**.

Second paragraph.
`
	n, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n.Title != "Monitor rollout" {
		t.Errorf("Title = %q", n.Title)
	}
	if n.Status != model.Doing {
		t.Errorf("Status = %q", n.Status)
	}
	if n.Priority != model.High {
		t.Errorf("Priority = %q", n.Priority)
	}
	if n.Category != "Project" {
		t.Errorf("Category = %q", n.Category)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "release" || n.Tags[1] != "ops" {
		t.Errorf("Tags = %v", n.Tags)
	}
	if n.Updated != "2026-07-19" {
		t.Errorf("Updated = %q", n.Updated)
	}
	if len(n.Links) != 1 || n.Links[0].URL != "https://example.com" || n.Links[0].Label != "Design doc" {
		t.Errorf("Links = %v", n.Links)
	}
	if !strings.HasPrefix(n.Body, "This is the **body**.") {
		t.Errorf("Body = %q", n.Body)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	n, err := Parse([]byte("just a body\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n.Title != "" || n.Body != "just a body\n" {
		t.Errorf("unexpected node: %+v", n)
	}
}

func TestParseUnterminated(t *testing.T) {
	if _, err := Parse([]byte("---\ntitle: x\nno closing delimiter\n")); err == nil {
		t.Error("expected error for unterminated frontmatter")
	}
}

func TestRoundTrip(t *testing.T) {
	orig := &model.Node{
		Title:    "Round trip",
		Status:   model.Todo,
		Priority: model.Low,
		Tags:     []string{"a"},
		Updated:  "2026-07-19",
		Links:    []model.Link{{URL: "https://x.test"}},
		Body:     "Body text.",
	}
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.HasSuffix(string(data), "Body text.\n") {
		t.Errorf("expected single trailing newline, got %q", string(data))
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Title != orig.Title || got.Status != orig.Status || got.Priority != orig.Priority {
		t.Errorf("scalars differ: %+v", got)
	}
	// Parse returns the body verbatim, including the single trailing newline
	// that Marshal normalizes to.
	if got.Body != "Body text.\n" {
		t.Errorf("Body = %q, want %q", got.Body, "Body text.\n")
	}
	if len(got.Links) != 1 || got.Links[0].URL != "https://x.test" {
		t.Errorf("Links = %v", got.Links)
	}
}

func TestMarshalOmitsEmpty(t *testing.T) {
	data, err := Marshal(&model.Node{Title: "Only title"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	if strings.Contains(s, "status:") || strings.Contains(s, "priority:") || strings.Contains(s, "links:") {
		t.Errorf("empty fields should be omitted, got:\n%s", s)
	}
}
