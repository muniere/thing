// Package search provides fuzzy lookup over the tree by title, slug, and tags.
package search

import (
	"sort"
	"strings"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// Result is one ranked match, ordered by descending score.
type Result struct {
	Type  model.NodeType `json:"type"`
	Slug  string         `json:"slug"`
	Title string         `json:"title"`
	Ref   string         `json:"ref"`
	Score int            `json:"score"`
}

// Find fuzzy-matches query against each node's title, slug, and tags, returning
// the matches ranked by score (ties broken by ref). An empty query matches
// everything.
func Find(entries []*store.Entry, query string) []Result {
	var results []Result
	for _, e := range entries {
		if e == nil || e.Node == nil {
			continue
		}
		n := e.Node
		score := best(fuzzyScore(n.Title, query), fuzzyScore(n.Slug, query))
		for _, tag := range n.Tags {
			score = best(score, fuzzyScore(tag, query))
		}
		if score < 0 {
			continue
		}
		results = append(results, Result{
			Type:  n.Type,
			Slug:  n.Slug,
			Title: n.Title,
			Ref:   e.Ref,
			Score: score,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Ref < results[j].Ref
	})
	return results
}

func best(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fuzzyScore returns a non-negative score when query matches s, or -1 when it
// does not. A contiguous substring scores higher than a scattered subsequence,
// and a match nearer the start scores highest. Matching is case-insensitive and
// works over runes, so it is safe for multibyte text.
func fuzzyScore(s, query string) int {
	if query == "" {
		return 1
	}
	s = strings.ToLower(s)
	q := strings.ToLower(query)

	if idx := strings.Index(s, q); idx >= 0 {
		// Positional decay, floored at 1 so a real substring is never dropped
		// as if it were a non-match even in a very long string.
		score := 100 - min(idx, 99)
		if idx == 0 {
			score += 50 // prefix bonus
		}
		return score
	}

	sr, qr := []rune(s), []rune(q)
	si, qi, score, prev := 0, 0, 0, -2
	for si < len(sr) && qi < len(qr) {
		if sr[si] == qr[qi] {
			if si == prev+1 {
				score += 5 // consecutive
			} else {
				score++
			}
			prev = si
			qi++
		}
		si++
	}
	if qi != len(qr) {
		return -1
	}
	return score
}
