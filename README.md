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

`thingd` serves the tree as a local web app: a React SPA (tree pane, full-edit
detail pane, status/category/tag filters) over a JSON API on the same Go data
layer as the CLI, so the two never disagree. One process hosts **multiple
projects** — each a named mount over its own data directory — registered in
`projects.yaml` and addressed under `/<project>`; the root `/` is a picker
listing them. The frontend lives in [`web/`](web/) (React + TypeScript, bundled
with esbuild; wire types are generated into `web/src/domain/generated.ts` from
`schema/tree.json` by `make gen` — see [`scripts/gen.sh`](scripts/gen.sh)).

### Projects

Projects are listed in `projects.yaml`, resolved (first match wins) from:

1. `$THING_DATA_DIR/projects.yaml`
2. `$XDG_STATE_HOME/thingd/projects.yaml`
3. `~/.local/state/thingd/projects.yaml`

```yaml
projects:
  - name: work            # URL-safe slug; addressed at /work
    dir: /path/to/work/.thing
  - name: home
    dir: /path/to/home/.thing
```

A name must be a URL-safe slug (`[a-z0-9-]`) and unique. A missing file is not an
error — the server starts with an empty picker.

An entry can also carry the display settings thingd applies to that board, with a
top-level `defaults` block for the ones every project shares — see
[Configuration](#configuration).

Projects can also be registered and unregistered at runtime — from the picker
page or via `PUT`/`DELETE /api/projects/<name>` (see the API below) — and the
change is written back to `projects.yaml` so it survives a restart. Registration
only mounts an **existing** thing tree (a directory that already holds a
`config.yaml`); create new ones with `thing init` first. Unregistering removes
the mount only and never deletes the data directory.

### Themes

Since one process serves several projects, each can pick its own palette — the
quickest way to tell at a glance which board you are looking at. Name one on the
project's entry in `projects.yaml` (see [Configuration](#configuration)):

```yaml
projects:
  - name: work
    dir: /path/to/work/.thing
    theme: teal   # amber (default), teal, violet, slate, crimson, forest
```

The root picker's **Edit** dialog offers the same choice, writing it back to
`projects.yaml`; the open board recolors itself over SSE without a reload. The
list it offers comes from the theme files themselves, so one you add appears
there too.

A theme is one stylesheet, served at `/themes/<name>.css`, that redefines the
design tokens under `[data-theme="<name>"]`. **Adding one is adding a
file** — nothing in `thingd` or the frontend enumerates the names that exist, so
no code change and no rebuild are involved. Two layers are read: the themes built
into the binary ([`web/themes/`](web/themes/)), then `themes/` under the same
state directory `projects.yaml` resolves to, so a `THING_DATA_DIR` holds a
complete thingd setup:

```
~/.local/state/thingd/
  projects.yaml
  themes/
    ocean.css      # a theme of your own
    teal.css       # ... or a few tokens layered over the built-in teal
```

Both layers contribute when both define a name, built-in first, so yours
overrides through the normal CSS cascade and only has to restate the tokens it
changes — a `teal.css` holding a single `--amber` recolors the built-in teal's
accent and leaves the rest. A name neither layer defines simply 404s and the
board keeps the default palette, which is also what a typo comes to. See
[`web/themes/README.md`](web/themes/README.md) for how to write one.

Every theme covers both the dark and the light color scheme. Which one applies
follows the reader's system by default; the `auto / light / dark` control in the
top bar fixes it instead, and the choice is written back to `projects.yaml` as a
top-level `defaults.scheme`, so it survives a restart and reaches every browser
on that server:

```yaml
defaults:
  scheme: dark   # or light; omit it to follow the system
```

It has no per-project counterpart on purpose: which scheme is comfortable is a
fact about the reader and the room, not about a project.

All of this affects `thingd` only; CLI output is never colored.

### Development

```
make serve                     # air rebuilds+restarts thingd on any change
make serve PORT=4400           # ... on a different port
```

`make serve` runs the dev loop through [air](https://github.com/air-verse/air):
it rebuilds the single embedded binary and restarts thingd on any Go **or**
frontend change, so dev runs the exact same one-binary, one-port app as
production. Open `http://localhost:4319`; Ctrl-C stops it. There is no separate
dev server or proxy — the browser reloads itself over SSE when thingd restarts
with a new build (thingd sends a per-process id in its `hello` frame and the
client reloads when it changes). Install air once with `go install
github.com/air-verse/air@latest`.

### Production

`make build` embeds the built SPA into `thingd`, so a single binary serves the
whole app with no external files:

```
make build
./bin/thingd                   # serves the app on http://localhost:4319
```

`--open` opens the browser; `--port N` pins the port; without it thingd falls
back to the next free port so several servers can run at once. Projects come from
`projects.yaml` (above). thingd embeds `web/dist` unconditionally, so a committed
`web/dist/.gitkeep` keeps `go build`/`test`/`vet` compiling before the first
`make build`.

### Daemon (`thing server`)

Running `./bin/thingd` holds a terminal in the foreground. To run it as a
background daemon instead, drive it from the `thing` CLI (the client/daemon split
mirrors `docker`/`dockerd`):

```
thing server start             # launch thingd in the background on port 4319
thing server start --port 4400 # ... on a different port
thing server start --open      # ... and open the browser
thing server status            # running (pid/port/url) + project count; exit 1 if stopped
thing server logs -f           # tail the daemon log; -n N shows the last N lines
thing server restart           # stop (if up) then start again
thing server stop              # graceful shutdown (SIGTERM), then remove state
```

There is a single global daemon on a fixed port (4319 by default; `start` errors
if one is already running). Its runtime state and log live next to
`projects.yaml` under the state directory (`~/.local/state/thingd/` by default):
`server.json` records the pid, port, and URL, and `thingd.log` collects the
daemon's output (appended across restarts). `thing server` locates the `thingd`
binary next to the running `thing`, then on `PATH`, overridable with `THINGD_BIN`
— so `make install` (which installs both) or a local `make build` both work.

### API

Project routes nest under `/api/projects/<project>/`, while `GET /api/projects`
lists the mounts for the picker. Within a project, every node is addressed by its
**ref** (a slug-path like `epic/issue/task`) used verbatim as the tail of the
path. Because a ref spans multiple path segments, per-field edits are carried in
a single `PATCH` body rather than as a path suffix.

```
GET    /api/projects                       registered projects (name, title, dir)
PUT    /api/projects/<p>                    register project <p> over {dir} (an existing thing tree); idempotent
PATCH  /api/projects/<p>                    reorder <p>: {before|after:"<name>"}, or edit it: {name|dir|theme}
DELETE /api/projects/<p>                    unregister <p> (leaves its data dir on disk)
GET    /api/projects/<p>/tree              whole tree as JSON (each node carries its ref)
POST   /api/projects/<p>/nodes/<parent>    create a child; the parent decides the type
PATCH  /api/projects/<p>/nodes/<ref>       {status|priority|title|category|body|move|addLink|removeLink|archive}
DELETE /api/projects/<p>/nodes/<ref>       remove (an epic/issue takes its subtree)
GET    /api/projects/<p>/archives          archived entries (ref, from, title, type, priority, status, archivedAt)
GET    /api/projects/<p>/archives/<name>   one archived entry's detail (from, status, body, ...)
PATCH  /api/projects/<p>/archives/<name>   restore _archives/<name> to where it came from, or {to:"<ref>"}
GET    /api/projects/<p>/events            Server-Sent Events reload stream (per project)
GET    /api/projects/<p>/config            display config (title, dir, filter defaults, theme)
GET    /api/projects/<p>/icon              the board's icon: the tree's own, else thing's mark
GET    /api/themes                         the theme names that exist, for the picker to offer
GET    /api/settings                       server-wide display config (color scheme)
PATCH  /api/settings                       set it: {scheme:"auto"|"light"|"dark"}
GET    /themes/<name>.css                  one theme's stylesheet, layered over the reader's own
```

Open browsers live-reload over SSE whenever that project's tree changes — whether
the edit came from the web, the CLI, or an editor — and a change in one project
never reloads another's browsers.

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
thing ls --archived                          # list only archived entries
thing ls --all                               # list the top level plus archived entries
thing show <ref>                             # show a node + body
thing status   <ref> <todo|doing|done|paused>
thing priority <ref> <high|medium|low>
thing mv <src> <dst>                         # move and/or rename a node (src/dst are refs)
thing rm <ref>                               # remove a node (an epic/issue takes its subtree)
thing archive <ref>                          # archive a node into _archives/ (takes its subtree)
thing unarchive <archive-ref> [--to <ref>]   # restore an archived node to the live tree
thing link add  <ref> <url> [--label <l>]    # add or update a related link
thing link rm   <ref> <url|index>            # remove a link by URL, or 1-based index
thing link list <ref>                        # list a node's related links
thing find <query> [--json]                  # fuzzy-search titles, slugs, and tags
thing check [<ref>]                          # validate body section convention; no ref = whole tree
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
lists that node's children; `thing ls _orphan` lists the orphan issues;
`thing ls --archived` lists the archived entries (see `archive` below).

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

### `archive` / `unarchive` — shelve and restore

`archive` moves a node out of the live tree into a hidden `_archives/` region (a
sibling of `_orphan/`), taking its whole subtree; a task takes only its file. The
ref it was archived from and the time are recorded in its frontmatter
(`archived_ref` / `archived_at`, an RFC3339 instant), and the archived entry is
addressed as `_archives/<name>` — printed by `archive` and listed by
`thing ls --archived`. Archived nodes drop out of `tree`, `ls`, `find`, and
`export`; reach one with `thing show _archives/<name>`.

`unarchive` restores an entry to its recorded `archived_ref`, or to `--to <ref>`
when given. Restoring onto an occupied ref, or one whose parent no longer exists, is
an error rather than an overwrite — retry with `--to`. When the restore lands
somewhere other than where it came from, `[[<ref>]]` backlinks (and their
descendants') are rewritten to the new ref, like `mv`.

While a node is archived its backlinks dangle (`[[<ref>]]` resolves to nothing),
the same as after `rm`. Avoid reusing an archived node's ref: if a new node takes
that ref, its `[[<ref>]]` references become its own, and restoring the archived node
elsewhere with `--to` leaves those references with the new occupant rather than
following the restored node.

```
thing archive alpha/one                    # shelve issue "one" and its tasks -> _archives/one
thing ls --archived                        # _archives/one  <- alpha/one  One  (2026-07-27T09:00:00+09:00)
thing unarchive _archives/one               # restore to alpha/one
thing unarchive _archives/one --to beta/one # restore under a different parent
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
  icon.svg                   # optional board icon (icon.png also works)
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

### Section convention

A body should carry four headings, at level 2 (`## `), in this order:
`Summary`, `Details`, and `Definition of Done` are required; `Comments` is
optional. Matching is case-insensitive with surrounding whitespace trimmed,
and tolerant of a closing ATX run (`## Summary ##`); a `# Summary` (level 1)
heading is not recognized, since a level-1 heading in a body is
conventionally the node's own title, never a section. A heading inside a
fenced code block is never recognized either.

A Japanese heading counts as the section it names, so a body written in
Japanese satisfies the convention without any English in it:

| Section              | Also accepted                            |
| -------------------- | ---------------------------------------- |
| `Summary`            | `概要`, `要約`, `サマリ`, `サマリー`      |
| `Details`            | `詳細`, `詳細説明`                       |
| `Definition of Done` | `完了条件`, `完了の定義`, `受入条件`, `DoD` |
| `Comments`           | `コメント`, `備考`                       |

Headings may mix languages within one body. The warnings stay English either
way — they name the convention, not the heading the body happened to use.

<!-- section-convention-example -->
```markdown
## Summary

Roll the new dashboard out to every region without a visible gap in alerting.

## Details

Region-by-region cutover, starting with the lowest-traffic region so a
misconfigured alert rule surfaces before it reaches production traffic.

## Definition of Done

- [ ] Every region is on the new dashboard
- [ ] No alert rule fired unexpectedly during cutover
```

This is a writing convention, not a schema: `thing` never writes a body back
out, and never rejects a body for not following it. `thing check [<ref>]`
validates a node's body against the convention and reports what it finds as
warnings — a missing required section, or sections out of order — never as
an error; its exit code is 0 either way. With no ref, it walks the whole tree
and prints only the nodes that have warnings:

```
$ thing check node-body-sections/comments
node-body-sections/comments
  warn: No Definition of Done section
  warn: Details appears before Summary
```

`thingd`'s web API (`ExportWeb`, not the plain `thing export` interchange
format) carries the same warnings as `markers`, each entry with its
`severity` (always `"warn"` today) and `message`.

## Configuration

Two files configure a board, split by what they describe.

`config.yaml`, in the tree's own data directory, holds what the tree **is**: the
board `title` and the `categories` used to group epics. The CLI reads it too, so
it is the same for everyone who works on the tree. See
[`config.example.yaml`](config.example.yaml).

A board also wears an icon of its own, in its tab and in the picker. It is found
by convention rather than named in `config.yaml`: drop an `icon.svg` (preferred,
since it stays sharp at every size) or an `icon.png` into the same data
directory, and that is the whole of the configuration. A tree carrying neither
gets thing's own ◉ mark, so the icon is worth setting mainly when several boards
are open at once and their tabs need telling apart.

`projects.yaml` holds what **thingd** does with it. Alongside each project's
`name` and `dir`, its entry carries the display settings that shape only the web
board — the `filter` state it starts from and the `theme` it renders in — and a
top-level `defaults` block supplies them for every project that does not:

```yaml
defaults:
  theme: amber
  filter:
    statuses: [todo, doing]   # every board opens on todo and doing only
    tag: wip
projects:
  - name: work
    dir: /path/to/work/.thing
    theme: teal               # ... this one is teal, not amber
    filter:
      tag: api                # ... and filters by tag api
  - name: home
    dir: /path/to/home/.thing
    filter:
      tag: null               # ... this one does not filter by tag
                              # (statuses is omitted, so it stays [todo, doing])
```

`theme` is a single choice rather than a set of keys, so it is not layered the way
`filter` is: an entry's value wins outright, and `defaults` supplies it only when
the entry sets none. See [Themes](#themes).

Filter keys mirror the sidebar facets (`statuses`, `priorities`, `category`,
`tag`, `query`); an omitted key means that facet is not filtered. An entry is
layered over `defaults` key by key, and an explicit null drops an inherited value
rather than inheriting it.

The defaults are the *starting* state, not a floor: they apply when the URL says
nothing about filters, and thingd then writes them into the query string so a
shared link shows the same board to everyone. Clearing every facet yields
`?filter=none`, which survives a reload; the sidebar's reset button returns to the
configured defaults.

## License

MIT — see [LICENSE](LICENSE).
