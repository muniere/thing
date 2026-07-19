// Package slug derives stable, filesystem-safe identifiers from node titles.
package slug

import (
	"strconv"
	"strings"
)

// Slugify converts a title into a lowercase, hyphen-separated slug: runs of
// characters outside [a-z0-9] collapse to a single hyphen, and leading and
// trailing hyphens are trimmed. An empty result falls back to "untitled".
func Slugify(title string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "untitled"
	}
	return s
}

// Unique returns base if it is not already taken, otherwise the first
// "base-N" (N starting at 2) that is free. The taken set is consulted but not
// modified; callers add the returned slug themselves.
func Unique(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !taken[candidate] {
			return candidate
		}
	}
}
