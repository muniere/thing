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
3. `-g` / `--global` → `$XDG_DATA_HOME/thing` (default `~/.local/share/thing`)
4. the nearest `.thing/` searched upward
5. otherwise an error — data is never taken from an implicit global location

### Config directory

1. `--config <path>`
2. `THING_CONFIG_DIR`
3. `-g` / `--global` → `$XDG_CONFIG_HOME/thing` (default `~/.config/thing`)
4. the nearest `.thing/` searched upward
5. otherwise `$XDG_CONFIG_HOME/thing` (default `~/.config/thing`)

A `.thing/` found upward holds both, so a self-contained project keeps its tree
and config together.

Run `thing init` to create the directories and a starter `config.yaml`. Like
`npm init`, a bare `thing init` anchors a new project at `./.thing` in the
current directory; `thing init -g` targets the global directories instead.

## Commands

The tree behaves like a filesystem: a node is addressed by a **slug path**
`<parent>/<name>`, and the commands mirror the shell's. A slug is globally
unique and carries its own type, so operations on an existing node take a bare
slug — there is no `epic`/`issue`/`task` prefix.

```
thing init                                   create the data + config dirs and config.yaml
thing add <path> [--priority <p>] [--tags a,b] [--category <c>]
thing ls [<parent>]                          # list a parent's children, or the top level
thing show <slug>                            # show a node + body
thing status   <slug> <todo|doing|done|paused>
thing priority <slug> <high|medium|low>
thing mv <src> <dst>                         # move and/or rename a node (like mv)
thing tree                                   # whole tree as an indented outline
```

### `add` — a path decides the type

`add` takes a path `[<parent>/]<title>`; the parent's type decides what is
created (like `mkdir` in a tree):

```
thing add "Web release"                 # no parent      -> epic
thing add web-release/"Monitor rollout" # under an epic  -> issue
thing add monitor-rollout/"Confirm"     # under an issue -> task
thing add _orphan/"Loose end"           # under _orphan  -> orphan issue
```

`--category` applies only to an epic. `add` prints the created slug on stdout.

### `ls` — list children

`thing ls` lists the top level (epics and orphan issues); `thing ls <parent>`
lists that node's children; `thing ls _orphan` lists the orphan issues.

### `mv` — move and/or rename

`mv` addresses a node by the same `<parent>/<name>` path — a bare `<name>` for
an epic, `_orphan/<name>` for an orphan issue. A changed parent moves the node;
a changed name renames it (rewriting `[[slug]]` backlinks across the tree);
changing both does both. The name is the node's slug, not its title. It is
silent on success.

```
thing mv alpha/one beta/one       # move issue "one" from epic alpha to beta
thing mv alpha/one alpha/planning # rename the slug one -> planning in place
thing mv alpha/one _orphan/one    # detach an issue into _orphan
```

The `--data-dir`, `--config`, and `-g` / `--global` flags apply to every
command.

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
