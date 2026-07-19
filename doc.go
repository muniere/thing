// Package thing is the root of the thing project: a topic outline
// (Epic > Issue > Task) stored as one Markdown file per node.
//
// The implementation lives under internal/ (shared Go data layer plus the thing and
// thingd binaries) and web/ (the thingd frontend). This file exists so the
// module has a buildable, testable root package from its first commit.
package thing
