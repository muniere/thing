---
name: thing
description: Use when reading or updating a thing topic tree (Epic > Issue > Task stored as Markdown files) from an agent. Covers reading the whole tree as JSON and mutating it with the `thing` CLI. Trigger when the user asks to list/inspect topics, add/move/remove an epic/issue/task, or change a node's status in a thing-managed directory.
---

# thing

`thing` manages a topic outline as one Markdown file per node under a data
directory. Hierarchy: **Epic > Issue > Task**. An Issue may live outside any Epic
(an "orphan", stored under `_orphan/`).

The tree behaves like a filesystem. A node is addressed by its **ref** — the
slash-joined path of slugs from the top: `<epic>`, `<epic>/<issue>`,
`<epic>/<issue>/<task>`, or `_orphan/<issue>` for an orphan. A slug is unique
only among its siblings, so the same name may appear under different parents; the
full ref is the identity.

## Data location

The data directory (the node tree) and the config directory (`config.yaml`)
resolve independently. For the **data** directory the order is:

1. `--data-dir <path>` flag
2. `THING_DATA_DIR` environment variable
3. `-g` / `--global` → the global data directory (`$XDG_DATA_HOME/thing`, else
   `~/.local/share/thing`)
4. the nearest `.thing/` searched upward from the working directory (git-style)

There is no implicit global fallback: with none of the above, commands error
rather than silently touch a global tree. Prefer passing `--data-dir` explicitly
in scripts so behavior is deterministic and independent of the working
directory. (`--config` / `THING_CONFIG_DIR` resolve the config directory the same
way, but fall back to the global config dir; most commands need only the data
directory.)

## Reading (agents: read via JSON)

To read the whole tree, use `export`, which prints JSON:

```
thing export --data-dir <path>
```

Output is an array of top-level nodes (epics and orphan issues). Each node:

```json
{
  "type": "epic|issue|task",
  "ref": "web-release/monitor-rollout",
  "title": "Monitor rollout",
  "status": "todo|doing|done|paused",
  "priority": "high|medium|low",   // optional
  "category": "Project",            // epics only, optional
  "tags": ["release"],              // optional
  "updated": "2026-07-20",          // optional
  "links": [                         // optional
    { "url": "https://...", "label": "Design doc" }
  ],
  "body": "free-form markdown",     // optional
  "children": [ /* nested nodes */ ]
}
```

`status` is the node's **own** status, present only when it is set; it is absent
when the node has none. A node with no own status has its displayed status rolled
up from its children (all done → done; any doing → doing; all todo → todo;
otherwise doing), recursively — an epic through its issues, an issue through its
tasks, a leaf task defaulting to todo. `export` carries only the own status, so
compute the rollup yourself if you need the displayed value; set a status to pin
it, or `thing status <ref> auto` to clear the pin. (thingd's web API additionally
emits a computed `effectiveStatus`; the CLI `export` does not.)

To find nodes by a query (fuzzy over title, slug, and tags):

```
thing find "<query>" --json --data-dir <path>
```

`find --json` returns a ranked array — each element is
`{ "type", "slug", "title", "ref", "score" }`. Use it to locate a **ref**, then
`export` or `show <ref>` for the details.

Human-readable listings (not JSON) are also available:

```
thing tree --data-dir <path>              # whole tree as an indented outline
thing ls [<ref>] --data-dir <path>        # a node's children, or the top level
thing ls --archived --data-dir <path>     # only archived entries (the hidden _archive/ region)
thing ls --all --data-dir <path>          # the top level plus archived entries
thing show <ref> --data-dir <path>        # one node incl. body
```

## Mutating

The state-change verbs (`status`, `priority`, `mv`, `rm`, `link add/rm`) are
silent on success (UNIX-style). `add` prints the new node's **ref** on stdout, so
capture it (slugs are derived from the title); `init` and `import` print their
own output too. So do not infer failure from output alone — check the exit code.

```
thing init --data-dir <path> --config <path>          # create dirs + starter config.yaml
thing add [<parent-ref>/]<title> [--priority high|medium|low] [--tags a,b] [--category <c>] --data-dir <path>
thing status   <ref> <todo|doing|done|paused|auto> --data-dir <path>   # 'auto' clears the pin -> rollup
thing priority <ref> <high|medium|low> --data-dir <path>
thing mv <src-ref> <dst-ref> --data-dir <path>        # move and/or rename; backlinks follow
thing rm <ref> --data-dir <path>                      # an epic/issue removes its whole subtree
thing archive   <ref> --data-dir <path>               # shelve into _archive/ (an epic/issue takes its subtree)
thing unarchive <_archive/name> [--to <ref>] --data-dir <path>  # restore an archived node to the live tree
thing link add  <ref> <url> [--label "<l>"] --data-dir <path>   # add or update a related link
thing link rm   <ref> <url|index> --data-dir <path>            # remove by URL, else by 1-based index
thing link list <ref> --data-dir <path>
```

`add` derives the node's type from its parent, like `mkdir` in a tree — no
parent → epic; under an epic → issue; under an issue → task; under `_orphan` →
orphan issue. `--category` applies only to an epic.

```
epic=$(thing add "Web release" --data-dir "$D")                 # -> web-release
issue=$(thing add "$epic/Monitor rollout" --data-dir "$D")      # -> web-release/monitor-rollout
task=$(thing add "$issue/Confirm routing" --data-dir "$D")      # -> web-release/monitor-rollout/confirm-routing
thing add '_orphan/Loose end' --data-dir "$D"                   # -> _orphan/loose-end
thing status "$task" done --data-dir "$D"
```

`mv` takes two refs, exactly like UNIX `mv`. A changed parent moves the node; a
changed final slug renames it; changing both does both. Renames rewrite `[[ref]]`
backlinks across the tree, including those of the node's descendants. The name is
the slug, not the title.

```
thing mv web-release/monitor-rollout ops/monitor-rollout   # move issue to epic "ops"
thing mv web-release/monitor-rollout web-release/rollout   # rename the slug in place
thing mv web-release/monitor-rollout _orphan/monitor-rollout  # detach into _orphan
```

## Archiving

`archive` moves a node out of the live tree into a hidden `_archive/` region (a
sibling of `_orphan/`); an epic or issue takes its whole subtree, a task takes
only its file. Archived nodes drop out of `export`, `tree`, `ls`, and `find`, so
they never appear in normal reads. The archived entry is addressed as
`_archive/<name>` (a machine-unique name, printed by `archive`), and its origin is
recorded in frontmatter — `archived_ref` (the live-tree ref it was archived from)
and `archived_at` (the RFC3339 instant). Reach one with `thing ls --archived` or
`thing show _archive/<name>`.

`unarchive` restores an entry to its recorded `archived_ref`, or to `--to <ref>`
when given. Restoring onto an occupied ref, or one whose parent no longer exists,
is an error (retry with `--to`); an existing node is never overwritten. Restoring
elsewhere rewrites `[[<ref>]]` backlinks to the new ref, like `mv`. While a node is
archived its backlinks dangle, so do not reuse an archived node's ref.

```
thing archive alpha/one --data-dir "$D"                     # -> _archive/one (issue "one" + its tasks)
thing ls --archived --data-dir "$D"                        # _archive/one  <- alpha/one  One  (2026-07-27T09:00:00+09:00)
thing unarchive _archive/one --data-dir "$D"               # restore to alpha/one
thing unarchive _archive/one --to beta/one --data-dir "$D" # restore under a different parent
```

## Bulk import

To create many nodes in one call, write a JSON **array** to a file and pass it to
`thing import`. The batch is a flat list (no `children`), so an `export` file is
not an import file. Each element is one node:
`{ "type": "task", "title": "...", "parent": "<ref>|inbox", "priority": ...,
"category": ..., "tags": [...], "links": [...], "body": "..." }`. `type` defaults
to `task`. `parent` is the **ref** of the parent node — or `"inbox"` for a task
(creates/reuses an orphan `inbox` issue), or empty for an epic or orphan issue
(an issue may also name `_orphan` explicitly). Only `title` is required. Items may
reference parents created earlier in the same batch.

```
thing import batch.json --data-dir <path>              # bulk-create
thing import batch.json --dry-run --data-dir <path>    # validate + assign refs, write nothing
```

It prints a result array (one entry per input item, in order) —
`{ "title", "ref", "parent", "status", "message" }`. `status` is `created`
(written), `validated` (accepted under `--dry-run`, nothing written), or `error`.
A failing item becomes `"status": "error"` without stopping the rest, and the
command exits non-zero if any item errored. `thing import` does no deduplication;
dedupe before calling it.

## Rules and error handling

- A slug is a stable ID (the directory/file name); a ref is the path of slugs.
  Move or rename via `mv`, never by editing the filesystem directly.
- Status values are exactly `todo`, `doing`, `done`, `paused` (or `auto` to clear
  a pin so the status rolls up from children); priorities are `high`, `medium`,
  `low`.
- Invalid input (unknown ref, missing/wrong-kind parent, bad status, unknown
  flag) exits non-zero with a message on stderr. Check the exit code; the
  state-change verbs are silent on success, so empty output is not a failure.
- `rm` on an epic/issue removes the whole subtree.
- `archive` hides a node under `_archive/` (out of `export`/`tree`/`ls`/`find`);
  read it with `ls --archived` / `show _archive/<name>` and restore with `unarchive`.
- A node's body is free-form Markdown; `[[<ref>]]` denotes a link to another
  node, and `mv` rewrites those backlinks automatically on a rename.

## Editing bodies directly

The per-node commands (`add`, `status`, `priority`, `link`) set frontmatter
(title/status/priority/category/tags/updated/links) but not the body — only
`import` can set a body at creation. To edit an existing node's body, edit its
Markdown file:
`<data-dir>/<epic>/_epic.md`, `.../<epic>/<issue>/_issue.md`, or
`.../<issue>/<task>.md` (orphan issues live under `<data-dir>/_orphan/`).
Preserve the frontmatter block delimited by `---`.
