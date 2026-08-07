// Package theme resolves a color theme name to the CSS that defines it.
//
// A theme is one file, <name>.css, holding CSS custom properties under
// :root[data-theme="<name>"]. Adding one means dropping a file in the theme
// directory — no code change, no rebuild, and nothing in thingd or the web
// frontend enumerates the names that exist.
//
// Two layers are read, in order: the themes built into the binary, then
// themes/ under thingd's state directory — the same directory projects.yaml
// resolves to, so a THING_DATA_DIR holds a complete thingd setup. Both layers
// contribute when both define a name, concatenated built-in first, so the
// reader's file overrides through the normal CSS cascade and only has to
// restate the tokens it changes.
package theme

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/muniere/thing/internal/registry"
)

// name matches a usable theme name. It is both a URL path segment and a file
// name, so anything that could climb out of the theme directory — or merely
// differ from it by case on a case-insensitive filesystem — is refused.
var name = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Loader reads themes from the built-in set plus the directory Dir reports.
type Loader struct {
	// Builtin holds the themes embedded in the binary, rooted at the directory
	// that contains them (so a theme is "<name>.css"). A nil FS is a Loader with
	// no built-in themes, which is valid: the reader's own layer still applies.
	Builtin fs.FS
}

// Read returns the CSS defining theme n, concatenated across both layers when
// both define it, and reports whether either did. An unknown or unsafe name is
// simply not found: the caller answers 404 and the board keeps its default
// palette, which is also what a typo in config.yaml should come to.
func (l Loader) Read(n string) ([]byte, bool) {
	if !name.MatchString(n) {
		return nil, false
	}
	var out [][]byte
	if l.Builtin != nil {
		if data, err := fs.ReadFile(l.Builtin, n+".css"); err == nil {
			out = append(out, data)
		}
	}
	if dir, err := Dir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, n+".css")); err == nil {
			out = append(out, data)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	// A newline between layers so a file that does not end in one cannot glue its
	// last rule to the next layer's first.
	return bytes.Join(out, []byte("\n")), true
}

// Dir returns the directory holding the reader's own themes: themes/ under
// thingd's state directory, beside projects.yaml. It is not required to exist —
// Read simply finds nothing in it — so one created later needs no restart.
//
// It resolves through registry.StateDir rather than a rule of its own, so the
// two thingd-only settings never disagree about where they live and pointing
// THING_DATA_DIR somewhere moves both.
func Dir() (string, error) {
	dir, err := registry.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes"), nil
}
