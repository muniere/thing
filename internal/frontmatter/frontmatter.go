// Package frontmatter parses and serializes a node file: a YAML frontmatter
// block delimited by "---" lines, followed by a free-form Markdown body.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/muniere/thing/internal/model"
	"gopkg.in/yaml.v3"
)

const delimiter = "---"

// doc mirrors the subset of model.Node that is persisted in frontmatter.
type doc struct {
	Title    string         `yaml:"title,omitempty"`
	Status   model.Status   `yaml:"status,omitempty"`
	Priority model.Priority `yaml:"priority,omitempty"`
	Category string         `yaml:"category,omitempty"`
	Tags     []string       `yaml:"tags,omitempty"`
	Updated  string         `yaml:"updated,omitempty"`
	Links    []model.Link   `yaml:"links,omitempty"`
}

// Parse reads a node file's raw bytes into a Node holding the frontmatter
// fields and the Markdown body. Type, Slug, and Children are left unset; the
// store layer fills those from the on-disk layout. Content without a leading
// frontmatter block is treated as an all-body node.
func Parse(data []byte) (*model.Node, error) {
	text := string(data)
	rest, ok := strings.CutPrefix(text, delimiter+"\n")
	if !ok {
		// No frontmatter block: the whole file is the body.
		return &model.Node{Body: text}, nil
	}
	end := strings.Index(rest, "\n"+delimiter)
	if end < 0 {
		return nil, fmt.Errorf("frontmatter: unterminated block (missing closing %q)", delimiter)
	}
	yamlPart := rest[:end]
	body := rest[end+len("\n"+delimiter):]
	// Drop the rest of the closing delimiter line and a single following newline.
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[i+1:]
	} else {
		body = ""
	}

	var d doc
	if err := yaml.Unmarshal([]byte(yamlPart), &d); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	return &model.Node{
		Title:    d.Title,
		Status:   d.Status,
		Priority: d.Priority,
		Category: d.Category,
		Tags:     d.Tags,
		Updated:  d.Updated,
		Links:    d.Links,
		Body:     body,
	}, nil
}

// Marshal serializes a node's frontmatter fields and body back into file bytes.
// The output always ends with a single trailing newline.
func Marshal(n *model.Node) ([]byte, error) {
	d := doc{
		Title:    n.Title,
		Status:   n.Status,
		Priority: n.Priority,
		Category: n.Category,
		Tags:     n.Tags,
		Updated:  n.Updated,
		Links:    n.Links,
	}
	yamlPart, err := yaml.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(delimiter + "\n")
	buf.Write(yamlPart)
	buf.WriteString(delimiter + "\n")
	body := strings.TrimRight(n.Body, "\n")
	if body != "" {
		buf.WriteString(body)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}
