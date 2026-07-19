# thing

`thing` manages a topic outline — **Epic > Issue > Task** — as one Markdown file
per node under a data directory. It ships two binaries that share a single Go
data layer:

- **`thing`** — an AI-facing CLI that reads and writes the tree directly.
- **`thingd`** — a human-facing web server (full CSR SPA + JSON API).

Slugs are the stable identity: a node's filename is its slug, and every ref is
slug-only (globally unique across the tree). There is no uid or ref-mode
machinery.

## Install

```
make build             # build all packages
```

## Node file format

Each node file is a YAML frontmatter block (delimited by `---`) followed by a
free-form Markdown body:

```markdown
---
title: Monitor rollout
status: doing
priority: high
category: Project        # epics only
tags:
  - release
updated: "2026-07-19"
links:
  - url: https://example.com
    label: Design doc
---
Free-form Markdown body. `[[other-slug]]` links to another node; `rename`
rewrites these backlinks automatically.
```

All frontmatter fields are optional. The filename is the slug — do not rename
files by hand; use `thing <res> rename`.

## License

MIT — see [LICENSE](LICENSE).
