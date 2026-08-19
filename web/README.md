# thing web frontend

The React + TypeScript SPA served by `thingd`. There is no Go here; the frontend
talks to thingd's project-scoped, ref-based JSON API (`/api/projects/<p>/…`) and
consumes the hand-written types in `src/domain/generated.ts`. See the root
[README](../README.md#web-thingd) for the project registry.

The URL says which of four things you are looking at:

| URL | |
|---|---|
| `/` | the project picker |
| `/<project>/` | that project's board, nothing focused |
| `/<project>/?ref=/<ref>` | the board, focused on that node |
| `/<project>/<ref>` | that node alone, given the whole window |

A path names a node — where it sits in the tree — while which node the board has
focused is part of the board's own state, written in the query beside its
filters. So a node's row on the board links to the node's path, and a plain click
is intercepted to focus it in place; ⌘/Ctrl/Shift/middle-clicking that same row
follows the link and opens the node on its own. `src/route.ts` holds the whole
contract.

[esbuild](https://esbuild.github.io/) bundles the SPA into `dist/` (see
`build.mjs`), which `thingd` embeds. There is no dev server: the dev loop runs
the one embedded binary under [air](https://github.com/air-verse/air), which
rebuilds it on any change, and the browser reloads itself over SSE (see
`src/live.ts`). So the same binary serves the app in dev and prod.

## Develop

From the repo root:

```
make serve                   # air rebuilds+restarts thingd on any change; open http://localhost:4319
make serve PORT=4400         # ... on a different port
```

Projects come from `projects.yaml` (see the root README), not a flag.

The page reloads whenever thingd restarts with a new build, and the tree also
live-reloads over SSE when the data changes from any source (web, CLI, editor).

## Scripts

```
npm run build       # type-check then build to dist/
npm run typecheck   # tsc --noEmit
```

Install deps with a plain `npm install` / `npm ci`. Do **not** pass
`--omit=optional` / `--no-optional`: esbuild's platform binary ships as an
optionalDependency, and the build fails without it.

## Production

From the repo root, `make build` builds `dist/` and embeds it into `thingd` for a
single self-contained binary that serves the whole app. See
the root [README](../README.md#web-thingd).
