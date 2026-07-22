# thing

`thing` manages a topic outline — **Epic > Issue > Task** — as one Markdown file
per node under a data directory. It ships two binaries that share a single Go
data layer:

- **`thing`** — an AI-facing CLI that reads and writes the tree directly.
- **`thingd`** — a human-facing web server (full CSR SPA + JSON API).

The tree is a filesystem: a node's directory or file name is its **slug**, and a
node is identified by its **ref** — a slash-joined slug path: `<epic>`,
`<epic>/<issue>`, `<epic>/<issue>/<task>` (an orphan issue lives under
`_orphan/`). A slug is unique only among its siblings, so the same name may recur
in different branches; the full ref is the identity. There is no uid.

## Install

```
make install
```

This installs `thing` into your Go bin directory (`go env GOBIN`, else
`$GOPATH/bin`); make sure it is on your `PATH`. Use `make build` to produce a
local `bin/thing` instead.

Shell completions live under [`completions/`](completions/): source
`thing.bash` from your `~/.bashrc`, or put `_thing` on your zsh `$fpath`.

## Web (thingd)

`thingd` serves the tree as a local web app: a JSON API over the same Go data
layer as the CLI, plus (in a later commit) the built SPA. The frontend lives in
[`web/`](web/) (React + Vite + TypeScript; types hand-written in
`web/src/domain/generated.ts`).

```
make build                     # build the CLI + thingd into bin/
./bin/thingd --dir <path>      # serve the API on http://localhost:4319
```

thingd resolves the data directory like the CLI, and without `--port` listens on
4319, falling back to the next free port so several trees can serve at once.
`--open` opens the browser; `--port N` pins the port (and errors if taken). The
frontend is still a shell that does not call the API yet — `make serve` runs the
Vite dev server on its own, and the integrated dev loop lands with the UI.

### API

Every node is addressed by its **ref** (a slug-path like `epic/issue/task`) used
verbatim as the URL path. Because a ref spans multiple path segments, per-field
edits are carried in a single `PATCH` body rather than as a path suffix.

```
GET    /api/tree                whole tree as JSON (each node carries its ref)
POST   /api/nodes/<parent>      create a child; the parent decides the type
PATCH  /api/nodes/<ref>         {status|priority|title|category|body|move|addLink|removeLink}
DELETE /api/nodes/<ref>         remove (an epic/issue takes its subtree)
```

The UI and SSE live-reload land in later commits.

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

The tree behaves like a filesystem: every node is addressed by its full **ref**,
and the commands mirror the shell's. There is no `epic`/`issue`/`task` prefix —
the ref already says what and where a node is.

```
thing init                                   create the data + config dirs and config.yaml
thing add [<parent>/]<title> [--priority <p>] [--tags a,b] [--category <c>]
thing ls [<ref>]                             # list a node's children, or the top level
thing show <ref>                             # show a node + body
thing status   <ref> <todo|doing|done|paused>
thing priority <ref> <high|medium|low>
thing mv <src> <dst>                         # move and/or rename a node (src/dst are refs)
thing rm <ref>                               # remove a node (an epic/issue takes its subtree)
thing link add  <ref> <url> [--label <l>]    # add or update a related link
thing link rm   <ref> <url|index>            # remove a link by URL, or 1-based index
thing link list <ref>                        # list a node's related links
thing find <query> [--json]                  # fuzzy-search titles, slugs, and tags
thing tree                                   # whole tree as an indented outline
thing export                                 # print the whole tree as JSON
thing import <file> [--dry-run]              # bulk-create nodes from a JSON batch
```

`find` fuzzy-matches the query against each node's title, slug, and tags,
ranking matches (a contiguous substring generally ranks above a scattered
subsequence; matches nearer the start rank highest). Plain output is
`<ref>  <title>  [<type>]` per line; `--json` emits a ranked array of
`{type, slug, title, ref, score}`.

### `add` — the parent decides the type

`add` takes `[<parent>/]<title>`; the parent's type decides what is created
(like `mkdir` in a tree). It prints the new node's ref.

```
thing add "Web release"                             # no parent      -> epic
thing add web-release/"Monitor rollout"             # under an epic  -> issue
thing add web-release/monitor-rollout/"Confirm"     # under an issue -> task
thing add _orphan/"Loose end"                       # under _orphan  -> orphan issue
```

`--category` applies only to an epic.

### `ls` — list children

`thing ls` lists the top level (epics and orphan issues); `thing ls <ref>`
lists that node's children; `thing ls _orphan` lists the orphan issues.

### `mv` — move and/or rename

`mv` takes two refs. A changed parent moves the node; a changed name
renames it (rewriting `[[ref]]` backlinks, including those of its descendants,
across the tree); changing both does both. The name is the node's slug, not its
title. It is silent on success.

```
thing mv alpha/one beta/one                # move issue "one" from epic alpha to beta
thing mv alpha/one alpha/planning          # rename the slug one -> planning in place
thing mv alpha/one/task beta/two/task      # move a task to another issue
thing mv alpha/one _orphan/one             # detach an issue into _orphan
```

The `--data-dir`, `--config`, and `-g` / `--global` flags apply to every
command.

A parent's status is rolled up from its children (all done → done; any doing →
doing; all todo → todo; otherwise doing) unless it sets a status explicitly: an
epic rolls up from its issues, an issue from its tasks, and the rollup recurses
so a statusless epic reflects its tasks through its issues.

### `export` / `import` — JSON in and out

`thing export` prints the whole tree as an indented JSON array of top-level
nodes, each with its subtree nested under `children` and its rolled-up status.

`thing import <file>` bulk-creates nodes from a JSON **array** (a flat list, not
nested). Each item is `{type, title, parent, priority, category, tags, links,
body}`; only `title` is required and `type` defaults to `task`. `parent` is the
ref of the parent node — or `"inbox"` for a task (creating/reusing an orphan
`inbox` issue), or empty for an epic or orphan issue. Items may reference
parents created earlier in the same batch. The batch is flat (no `children`), so
an `export` file is **not** an import file.

It prints a JSON result array (one entry per item, in order) of
`{title, ref, parent, status, message}`; a failing item becomes
`"status": "error"` without stopping the rest, and the command exits non-zero if
any item failed. `--dry-run` validates parents and assigns refs without writing
(each accepted item reports `"status": "validated"`).

```
thing export > tree.json
thing import batch.json                      # bulk-create; exits non-zero on any item error
thing import batch.json --dry-run            # validate only, write nothing
```

### Categories

`config.yaml` may list `categories` — headings used to group epics in `thing
tree` and top-level `thing ls`, in the listed order. Set an epic's category with
`thing add "<title>" --category <c>`; each epic belongs to at most one. Epics
with an empty or unknown category, and all orphan issues, fall under
`(uncategorized)`. With no categories configured, output stays flat.

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
Free-form Markdown body. `[[<ref>]]` links to another node by its ref;
`mv` rewrites these backlinks automatically when a node moves or is renamed.
```

All frontmatter fields are optional. A node's name on disk is its slug — do not
move or rename files by hand; use `thing mv`.

## Configuration

`config.yaml` holds the board `title` and the `categories` used to group epics.
See [`config.example.yaml`](config.example.yaml).

## License

MIT — see [LICENSE](LICENSE).
