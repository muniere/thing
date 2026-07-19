package slug

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Web staged release":       "web-staged-release",
		"Monitor 10% rollout":      "monitor-10-rollout",
		"  Leading/trailing  ":     "leading-trailing",
		"Already-a-slug":           "already-a-slug",
		"UPPER_snake.case":         "upper-snake-case",
		"日本語 title":                "title",
		"!!!":                      "untitled",
		"":                         "untitled",
		"multiple   spaces & dash": "multiple-spaces-dash",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnique(t *testing.T) {
	taken := map[string]bool{"foo": true, "foo-2": true, "bar": true}

	if got := Unique("baz", taken); got != "baz" {
		t.Errorf("free slug: got %q, want baz", got)
	}
	if got := Unique("foo", taken); got != "foo-3" {
		t.Errorf("collision: got %q, want foo-3", got)
	}
	if got := Unique("bar", taken); got != "bar-2" {
		t.Errorf("single collision: got %q, want bar-2", got)
	}
	// Unique must not mutate the taken set.
	if taken["baz"] {
		t.Error("Unique mutated the taken set")
	}
}
