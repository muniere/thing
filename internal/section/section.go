// Package section validates a node body against the section convention. It
// exists because thing never writes a body back out: people and AIs edit node
// files directly, and the convention headings are a writing convention kept
// by the "thing" skill, not enforced by this tool. An earlier attempt at this
// feature gave a parsed body both a read side and a write side (rendering it
// back to Markdown, and exposing its parsed sections for display); every bug
// it produced came from restructuring how the body was shown — content
// disappearing, content duplicated, a body with no Details section rendering
// empty. This package has no write side and hands out no parsed content at
// all — only warnings — so that bug class cannot exist here.
package section

import (
	"regexp"
	"strings"
)

// Marker is one warning from validating a body against the section
// convention.
type Marker struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// convention is the prescribed heading order, canonically named (trimmed,
// lowercased). Comments is the only optional one.
var convention = []string{"summary", "details", "definition of done", "comments"}

// required marks which convention headings must be present.
var required = map[string]bool{
	"summary":            true,
	"details":            true,
	"definition of done": true,
}

// display restores a convention heading's canonical spelling, keyed by its
// canonical (trimmed, lowercased) form, for use in a Marker's Message.
var display = map[string]string{
	"summary":            "Summary",
	"details":            "Details",
	"definition of done": "Definition of Done",
	"comments":           "Comments",
}

// rank reports name's position in convention, or -1 when name is not one of
// the convention headings.
func rank(name string) int {
	for i, c := range convention {
		if c == name {
			return i
		}
	}
	return -1
}

// headingRe recognizes an ATX heading, absorbing leading whitespace, a
// closing ATX suffix ("## Summary ##"), and trailing whitespace. Group 1 is
// the hash run (its length is the heading level); group 2 is the heading
// text. Only a level-2 run (exactly "##") is a recognized heading — a
// level-1 heading is conventionally the node's own title, and a level-3 (or
// deeper) heading is never part of the convention — so a caller must check
// len(group 1) == 2.
var headingRe = regexp.MustCompile(`^\s*(#+)\s*(.+?)\s*#*\s*$`)

// fenceRe recognizes the opening or closing delimiter of a fenced code
// block. A heading inside a fence is never recognized as a heading.
var fenceRe = regexp.MustCompile("^\\s*(`{3,}|~{3,})")

// eachUnfencedLine calls visit for every line outside a fenced code block, in
// file order. Fence state is tracked by character and run length, mirroring
// CommonMark: a fence closes only on a delimiter of the same character with a
// run at least as long as the one that opened it. A fence left unclosed at
// end of input simply swallows the rest of the lines; visit never sees them.
func eachUnfencedLine(lines []string, visit func(line string)) {
	inFence := false
	var fenceChar byte
	var fenceLen int

	for _, line := range lines {
		if inFence {
			if m := fenceRe.FindStringSubmatch(line); m != nil && m[1][0] == fenceChar && len(m[1]) >= fenceLen {
				inFence = false
			}
			continue
		}
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			inFence = true
			fenceChar = m[1][0]
			fenceLen = len(m[1])
			continue
		}
		visit(line)
	}
}

// canon normalizes a heading for matching against the convention names: trim
// surrounding whitespace, then lowercase.
func canon(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// scanHeadings reports the canonical name of every recognized ("## ",
// level-2 only) heading found outside fenced code blocks, in file order,
// including a repeat of one already seen.
func scanHeadings(body string) []string {
	var names []string
	eachUnfencedLine(strings.Split(body, "\n"), func(line string) {
		m := headingRe.FindStringSubmatch(line)
		if m == nil || len(m[1]) != 2 {
			return
		}
		names = append(names, canon(m[2]))
	})
	return names
}

// Check validates body against the section convention — Summary, Details,
// and Definition of Done required, Comments optional, all as "## " headings
// in that order — and reports what it finds as warnings. It never errors:
// the convention is a writing guideline the "thing" skill teaches, not a
// schema this tool enforces, so a body that ignores it entirely just yields
// one "missing" warning per required heading. Returns nil, not an empty
// slice, when there is nothing to report.
func Check(body string) []Marker {
	// found holds each recognized convention heading's canonical name on its
	// first occurrence, in file order; a repeat of a heading already seen
	// does not get a second entry.
	seen := make(map[string]bool, len(convention))
	var found []string
	for _, name := range scanHeadings(body) {
		if rank(name) < 0 || seen[name] {
			continue
		}
		seen[name] = true
		found = append(found, name)
	}

	var markers []Marker
	for _, name := range convention {
		if required[name] && !seen[name] {
			markers = append(markers, Marker{Severity: "warn", Message: "No " + display[name] + " section"})
		}
	}
	for i := 1; i < len(found); i++ {
		prev, cur := found[i-1], found[i]
		if rank(cur) < rank(prev) {
			markers = append(markers, Marker{
				Severity: "warn",
				Message:  display[prev] + " appears before " + display[cur],
			})
		}
	}
	return markers
}
