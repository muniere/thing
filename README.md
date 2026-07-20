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
make install
```

This installs `thing` into your Go bin directory (`go env GOBIN`, else
`$GOPATH/bin`); make sure it is on your `PATH`. Use `make build` to produce a
local `bin/thing` instead.

## Directories

`thing` keeps the node tree in a **data** directory and `config.yaml` in a
**config** directory. Each resolves independently — the first match wins.

### Data directory

1. `--data-dir <path>`
2. `THING_DATA_DIR`
3. the nearest `.thing/` searched upward (skipped with `-g` / `--global`)
4. `$XDG_DATA_HOME/thing` (default `~/.local/share/thing`)

### Config directory

1. `--config <path>`
2. `THING_CONFIG_DIR`
3. the nearest `.thing/` searched upward (skipped with `-g` / `--global`)
4. `$XDG_CONFIG_HOME/thing` (default `~/.config/thing`)

A `.thing/` found upward holds both, so a self-contained project keeps its tree
and config together.

Run `thing init` to create the directories and a starter `config.yaml`.

## Commands

```
thing init                                   create the data + config dirs and config.yaml
thing epic   add "<title>" [--category <c>] [--priority <p>] [--tags a,b]
thing issue  add "<title>" [--epic <slug>]  [--priority <p>] [--tags a,b]   # no --epic → orphan
thing task   add "<title>"  --issue <slug>  [--priority <p>] [--tags a,b]
thing epic   list                            # list epics
thing issue  list [--epic <slug>]            # list issues (optionally scoped)
thing task   list [--issue <slug>]           # list tasks (optionally scoped)
thing epic   show <slug>                     # show an epic + body
thing issue  show <slug>                     # show an issue + body
thing task   show <slug>                     # show a task + body
thing tree                                   # whole tree as an indented outline
```

The `--data-dir`, `--config`, and `-g` / `--global` flags apply to every
command. `add` prints the created slug on stdout.

An epic's status is rolled up from its issues (all done → done; any doing →
doing; all todo → todo; otherwise doing) unless the epic sets a status
explicitly.

## On-disk layout

```
<config-dir>/
  config.yaml
<data-dir>/
  <epic-slug>/
    _epic.md
    <issue-slug>/
      _issue.md
      <task-slug>.md
  _orphan/
    <issue-slug>/            # issues that belong to no epic
      _issue.md
      <task-slug>.md
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

## Configuration

`config.yaml` holds the board `title`. See
[`config.example.yaml`](config.example.yaml).

## License

MIT — see [LICENSE](LICENSE).
