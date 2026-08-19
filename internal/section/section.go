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

// section is one heading of the convention: the spelling a Marker names it
// by, whether a body must carry it, and the other spellings that count as it.
type section struct {
	label    string
	required bool
	aliases  []string
}

// convention is the prescribed heading order — a section's index is its rank,
// which is what "out of order" is measured against — carrying everything known
// about a heading on its own row. Adding a heading is one row and accepting
// another spelling of one is one word, with nothing to keep in step elsewhere.
// Comments is the only optional heading. Bodies are written in Japanese as
// often as in English, so a Japanese heading counts as the section it names;
// the Markers stay English, since they describe the convention rather than the
// body.
var convention = []section{
	{label: "Summary", required: true, aliases: []string{"概要", "要約", "サマリ", "サマリー"}},
	{label: "Details", required: true, aliases: []string{"詳細", "詳細説明"}},
	{label: "Definition of Done", required: true, aliases: []string{"完了条件", "完了の定義", "受入条件", "DoD"}},
	{label: "Comments", aliases: []string{"コメント", "備考"}},
}

// ranks resolves a heading, in the canonical (trimmed, lowercased) form canon
// produces, to its rank — its index in convention. It holds every spelling of
// every heading, the label and the aliases alike, and is derived from
// convention rather than written out so the two can never disagree. The
// lowercasing here is what lets convention spell a heading the way a body
// would ("Definition of Done", "DoD") instead of pre-canonicalized.
var ranks = func() map[string]int {
	m := make(map[string]int, len(convention))
	for i, s := range convention {
		for _, spelling := range append([]string{s.label}, s.aliases...) {
			m[strings.ToLower(spelling)] = i
		}
	}
	return m
}()

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

// canon normalizes a heading into the form ranks is keyed by: trim
// surrounding whitespace, then lowercase. TrimSpace trims by Unicode class, so
// an ideographic space ("## 概要　") goes too.
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
	// found holds the rank of each recognized convention heading on its first
	// occurrence, in file order; a repeat of a heading already seen does not
	// get a second entry, and neither does a second spelling of it.
	seen := make(map[int]bool, len(convention))
	var found []int
	for _, name := range scanHeadings(body) {
		i, ok := ranks[name]
		if !ok || seen[i] {
			continue
		}
		seen[i] = true
		found = append(found, i)
	}

	var markers []Marker
	for i, s := range convention {
		if s.required && !seen[i] {
			markers = append(markers, Marker{Severity: "warn", Message: "No " + s.label + " section"})
		}
	}
	for i := 1; i < len(found); i++ {
		prev, cur := found[i-1], found[i]
		if cur < prev {
			markers = append(markers, Marker{
				Severity: "warn",
				Message:  convention[prev].label + " appears before " + convention[cur].label,
			})
		}
	}
	return markers
}
