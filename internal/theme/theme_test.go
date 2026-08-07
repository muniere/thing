package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// builtin returns a Loader whose built-in layer holds one teal theme, with the
// state directory pointed at an empty temp dir so the developer's own themes
// never leak into a test.
func builtin(t *testing.T) Loader {
	t.Helper()
	t.Setenv("THING_DATA_DIR", t.TempDir())
	return Loader{Builtin: fstest.MapFS{"teal.css": {Data: []byte("/*builtin*/")}}}
}

// write drops <name>.css holding body into the state directory's themes/.
func write(t *testing.T, name, body string) {
	t.Helper()
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".css"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadBuiltin(t *testing.T) {
	css, ok := builtin(t).Read("teal")
	if !ok {
		t.Fatal("Read(teal) = not found, want the built-in theme")
	}
	if string(css) != "/*builtin*/" {
		t.Errorf("css = %q, want the built-in body", css)
	}
}

func TestReadUnknownName(t *testing.T) {
	if _, ok := builtin(t).Read("nope"); ok {
		t.Error("Read(nope) = found, want not found")
	}
}

// A theme defined in both layers is concatenated built-in first, so the reader's
// file wins through the normal CSS cascade and only has to restate the tokens it
// changes.
func TestReadLayersCascade(t *testing.T) {
	l := builtin(t)
	write(t, "teal", "/*reader*/")

	css, ok := l.Read("teal")
	if !ok {
		t.Fatal("Read(teal) = not found")
	}
	at, after := strings.Index(string(css), "/*builtin*/"), strings.Index(string(css), "/*reader*/")
	if at < 0 || after < 0 {
		t.Fatalf("css = %q, want both layers", css)
	}
	if at > after {
		t.Errorf("css = %q, want the built-in layer first", css)
	}
}

// A name only the reader's directory defines is a theme like any other: adding
// one takes no code change and no rebuild.
func TestReadReaderOnlyTheme(t *testing.T) {
	l := builtin(t)
	write(t, "ocean", "/*ocean*/")
	css, ok := l.Read("ocean")
	if !ok {
		t.Fatal("Read(ocean) = not found, want the reader's own theme")
	}
	if !strings.Contains(string(css), "/*ocean*/") {
		t.Errorf("css = %q, want the reader's body", css)
	}
}

// Names are the tail of a URL path and a file name, so anything that could climb
// out of the theme directory or name a file outside it is refused outright.
func TestReadRejectsUnsafeNames(t *testing.T) {
	l := builtin(t)
	for _, name := range []string{"", "..", "../teal", "sub/teal", "Teal", "teal.css", "teal;", ".teal"} {
		if _, ok := l.Read(name); ok {
			t.Errorf("Read(%q) = found, want refused", name)
		}
	}
}

// Themes sit beside projects.yaml, so they resolve through exactly the same rule
// — including THING_DATA_DIR, which is what makes a self-contained thingd state
// directory possible.
func TestDirFollowsStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv("THING_DATA_DIR", state)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(state, "themes"); dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}

	t.Setenv("THING_DATA_DIR", "")
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	dir, err = Dir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(xdg, "thingd", "themes"); dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

// List is what lets a picker offer themes without anything enumerating them in
// code: it reports the union of both layers, so a file the reader drops in shows
// up as a choice.
func TestList(t *testing.T) {
	l := builtin(t)
	write(t, "ocean", "/*ocean*/")
	write(t, "teal", "/*override*/") // also built in — must not appear twice

	got := l.List()
	want := []string{"ocean", "teal"}
	if len(got) != len(want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List() = %v, want %v (sorted, deduped)", got, want)
		}
	}
}

// A directory that does not exist is not an error — it just contributes nothing.
func TestListWithoutReaderDir(t *testing.T) {
	if got := builtin(t).List(); len(got) != 1 || got[0] != "teal" {
		t.Errorf("List() = %v, want just the built-in teal", got)
	}
}

// Only .css files whose names are usable count; anything else in the directory is
// ignored rather than offered as a theme that cannot resolve.
func TestListIgnoresNonThemes(t *testing.T) {
	l := builtin(t)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "Ocean.css", ".hidden.css", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := l.List(); len(got) != 1 || got[0] != "teal" {
		t.Errorf("List() = %v, want just the built-in teal", got)
	}
}
