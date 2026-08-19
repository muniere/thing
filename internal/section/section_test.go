package section

import (
	"os"
	"strings"
	"testing"
)

// wellFormedBody carries all four convention headings, in order.
const wellFormedBody = `## Summary

A short summary.

## Details

More detail.

## Definition of Done

- [ ] pending item
- [x] done item

## Comments

### //2026-01-02

Second comment.

### //2026-01-01

First comment.
`

func TestCheckWellFormedBody(t *testing.T) {
	if got := Check(wellFormedBody); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckCommentsOptional guards that a body missing only the optional
// Comments section reports nothing.
func TestCheckCommentsOptional(t *testing.T) {
	body := "## Summary\n\nA short summary.\n\n## Details\n\nMore detail.\n\n## Definition of Done\n\n- [ ] item\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

func TestCheckMissingRequiredSections(t *testing.T) {
	got := Check("Just some text, no headings at all.\n")
	want := []Marker{
		{Severity: "warn", Message: "No Summary section"},
		{Severity: "warn", Message: "No Details section"},
		{Severity: "warn", Message: "No Definition of Done section"},
	}
	if len(got) != len(want) {
		t.Fatalf("Check() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Check()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestCheckOneMissingRequiredSection guards that only the missing heading is
// reported when the others are present.
func TestCheckOneMissingRequiredSection(t *testing.T) {
	body := "## Summary\n\nA short summary.\n\n## Details\n\nMore detail.\n"
	got := Check(body)
	if len(got) != 1 || got[0].Message != "No Definition of Done section" {
		t.Errorf("Check() = %+v, want a single missing-Definition-of-Done warning", got)
	}
}

func TestCheckOutOfOrder(t *testing.T) {
	body := "## Details\n\nMore detail.\n\n## Summary\n\nA short summary.\n\n## Definition of Done\n\n- [ ] item\n"
	got := Check(body)
	found := false
	for _, m := range got {
		if m.Message == "Details appears before Summary" {
			found = true
		}
	}
	if !found {
		t.Errorf("Check() = %+v, want a warning that Details appears before Summary", got)
	}
}

// TestCheckEmptyBody guards that an empty body is treated the same as one
// with no recognized headings: every required section is reported missing.
func TestCheckEmptyBody(t *testing.T) {
	got := Check("")
	if len(got) != 3 {
		t.Fatalf("Check(\"\") = %+v, want 3 missing-section warnings", got)
	}
}

// TestCheckNonConventionHeadingsIgnored guards that a body using only
// non-canonical "## " headings is treated like one with no recognized
// headings at all — not silently accepted because it has headings.
func TestCheckNonConventionHeadingsIgnored(t *testing.T) {
	body := "## 決定事項\n\n- some decision\n\n## 実装\n\nsome text\n"
	got := Check(body)
	if len(got) != 3 {
		t.Fatalf("Check() = %+v, want 3 missing-section warnings", got)
	}
}

// TestCheckFencedHeadingsIgnored guards the one rule the epic calls
// non-negotiable: a "## " line inside a fenced code block is never a
// heading. This repo's own body for this feature has exactly this shape, a
// ```markdown block containing "## Summary" as an example.
func TestCheckFencedHeadingsIgnored(t *testing.T) {
	body := "## Summary\n" +
		"\n" +
		"A short summary.\n" +
		"\n" +
		"## Details\n" +
		"\n" +
		"```markdown\n" +
		"## Definition of Done\n" +
		"```\n" +
		"\n" +
		"Text after the fence.\n"
	got := Check(body)
	if len(got) != 1 || got[0].Message != "No Definition of Done section" {
		t.Errorf("Check() = %+v, want the fenced heading ignored (still missing)", got)
	}
}

// TestCheckTildeFence guards ~~~ fences the same way ``` fences are guarded.
func TestCheckTildeFence(t *testing.T) {
	body := "## Summary\n" +
		"\n" +
		"~~~\n" +
		"## Details\n" +
		"~~~\n" +
		"\n" +
		"## Details\n" +
		"\n" +
		"Real details.\n" +
		"\n" +
		"## Definition of Done\n" +
		"\n" +
		"- [ ] item\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckUnclosedFence guards that a fence left open to end of input
// swallows the rest of the body instead of misreading a later "## " line as
// a heading.
func TestCheckUnclosedFence(t *testing.T) {
	body := "## Summary\n" +
		"\n" +
		"```\n" +
		"## Details\n" +
		"note: fence never closes\n"
	got := Check(body)
	if len(got) != 2 {
		t.Fatalf("Check() = %+v, want Details and Definition of Done both reported missing", got)
	}
}

// TestCheckHeadingLeniency guards the heading recognizer's absorption rules:
// leading whitespace before "##" and a closing ATX suffix ("## Details ##").
func TestCheckHeadingLeniency(t *testing.T) {
	body := "  ## Summary\n" +
		"\n" +
		"Indented heading.\n" +
		"\n" +
		"## Details ##\n" +
		"\n" +
		"Closed ATX heading.\n" +
		"\n" +
		"## Definition of Done\n" +
		"\n" +
		"- [ ] item\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckLevelOneAndThreeIgnored guards that only a level-2 "## " heading
// is recognized: a level-1 "# Summary" is silently ignored (it is
// conventionally the node's own title), and a level-3 "### " heading (as
// used for Comments entries) is never mistaken for a convention section.
func TestCheckLevelOneAndThreeIgnored(t *testing.T) {
	body := "# Summary\n\nThis is a title, not a section.\n\n### Details\n\nThis is not level 2 either.\n"
	got := Check(body)
	if len(got) != 3 {
		t.Fatalf("Check() = %+v, want all 3 required sections reported missing", got)
	}
}

// TestCheckCaseInsensitive guards that matching ignores case.
func TestCheckCaseInsensitive(t *testing.T) {
	body := "## summary\n\nA.\n\n## DETAILS\n\nB.\n\n## definition of DONE\n\nC.\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckJapaneseHeadings guards that a body written with Japanese headings
// satisfies the convention exactly as an English one does.
func TestCheckJapaneseHeadings(t *testing.T) {
	body := "## 概要\n\n短い概要。\n\n## 詳細\n\nもっと詳しく。\n\n## 完了条件\n\n- [ ] 項目\n\n## コメント\n\n### //2026-01-01\n\nひとこと。\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckAliasVariants guards that every alias in the convention table
// actually resolves, by substituting it into an otherwise English body one
// section at a time. It walks the table rather than a copy of it, so a
// spelling added there is covered without touching this test. An alias that
// stops resolving is a silent regression: the only symptom is a spurious
// missing-section warning on a body that reads fine.
func TestCheckAliasVariants(t *testing.T) {
	for i, section := range convention {
		for _, spelling := range section.aliases {
			t.Run(section.label+"/"+spelling, func(t *testing.T) {
				var body strings.Builder
				for j, s := range convention {
					heading := s.label
					if j == i {
						heading = spelling
					}
					body.WriteString("## " + heading + "\n\nX.\n\n")
				}
				if got := Check(body.String()); got != nil {
					t.Errorf("Check() = %+v, want nil", got)
				}
			})
		}
	}
}

// TestRanksWellFormed guards the convention table itself, since a mistake
// there fails open: a spelling listed under two sections resolves to whichever
// row the loop reached last, and one listed twice under the same section hides
// a typo in the other. Either way the symptom is only that a heading stops
// being recognized, so the table is checked directly.
func TestRanksWellFormed(t *testing.T) {
	for i, section := range convention {
		if section.label == "" {
			t.Errorf("convention[%d]: no label", i)
		}
		for _, spelling := range append([]string{section.label}, section.aliases...) {
			if got, ok := ranks[strings.ToLower(spelling)]; !ok || got != i {
				t.Errorf("ranks[%q] = %d (present: %t), want %d — spelling claimed by another section?",
					spelling, got, ok, i)
			}
		}
	}

	total := 0
	for _, section := range convention {
		total += 1 + len(section.aliases)
	}
	if len(ranks) != total {
		t.Errorf("len(ranks) = %d, want %d — a spelling is listed twice", len(ranks), total)
	}
}

// TestCheckMixedLanguageHeadings guards that English and Japanese headings mix
// freely within one body — a body part-translated mid-edit is common, and each
// heading resolves on its own.
func TestCheckMixedLanguageHeadings(t *testing.T) {
	body := "## Summary\n\nA.\n\n## 詳細\n\nB.\n\n## Definition of Done\n\n- [ ] item\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// TestCheckJapaneseMarkersStayEnglish guards that the Markers describe the
// convention, not the body: a Japanese body out of order and missing a
// section still reports the canonical English names.
func TestCheckJapaneseMarkersStayEnglish(t *testing.T) {
	body := "## 詳細\n\nB.\n\n## 概要\n\nA.\n"
	got := Check(body)
	want := []Marker{
		{Severity: "warn", Message: "No Definition of Done section"},
		{Severity: "warn", Message: "Details appears before Summary"},
	}
	if len(got) != len(want) {
		t.Fatalf("Check() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Check()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestCheckIdeographicSpaceTrimmed guards that a heading padded with the
// ideographic space (U+3000) a Japanese input method produces still matches —
// the regexp's \s does not cover it, so canon's trim has to.
func TestCheckIdeographicSpaceTrimmed(t *testing.T) {
	body := "## 概要　\n\nA.\n\n## 　詳細\n\nB.\n\n## 完了条件　\n\n- [ ] item\n"
	if got := Check(body); got != nil {
		t.Errorf("Check() = %+v, want nil", got)
	}
}

// docsExampleMarker precedes the fenced example this package's doc tests
// extract; both README.md and skills/thing/SKILL.md carry other fenced
// blocks (YAML config, shell) that must not be picked up instead.
const docsExampleMarker = "<!-- section-convention-example -->"

// extractExampleFence pulls the fenced code block immediately following
// docsExampleMarker out of path, and returns its contents (without the
// fence lines themselves). It fails the test outright — rather than
// skipping — if the file is missing or the marker is not found, since a
// silently-skipped doc test defeats the purpose of keeping the docs honest.
func extractExampleFence(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")

	markerAt := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == docsExampleMarker {
			markerAt = i
			break
		}
	}
	if markerAt < 0 {
		t.Fatalf("%s: marker %q not found", path, docsExampleMarker)
	}

	fenceStart := -1
	for i := markerAt + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			fenceStart = i
			break
		}
	}
	if fenceStart < 0 {
		t.Fatalf("%s: no fenced block after marker at line %d", path, markerAt+1)
	}

	fenceEnd := -1
	for i := fenceStart + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			fenceEnd = i
			break
		}
	}
	if fenceEnd < 0 {
		t.Fatalf("%s: fenced block opened at line %d never closes", path, fenceStart+1)
	}

	return strings.Join(lines[fenceStart+1:fenceEnd], "\n")
}

// TestDocsExamplesParse keeps README.md and skills/thing/SKILL.md honest:
// each carries a worked example body, marked with docsExampleMarker so this
// test can find it among the surrounding YAML and shell fences, and both
// must actually pass Check cleanly. A documented example that would itself
// trigger a warning is a documentation bug.
func TestDocsExamplesParse(t *testing.T) {
	for _, path := range []string{"../../README.md", "../../skills/thing/SKILL.md"} {
		t.Run(path, func(t *testing.T) {
			body := extractExampleFence(t, path)
			if got := Check(body); got != nil {
				t.Errorf("Check() = %+v, want nil", got)
			}
		})
	}
}
