# thing web frontend

The React + Vite + TypeScript SPA served by `thingd`. There is no Go here; the
frontend talks to thingd's ref-based JSON API and consumes the hand-written
types in `src/domain/generated.ts`.

Vite is only the bundler — it builds `dist/`, which `thingd` embeds. There is no
Vite dev server: the dev loop runs the one embedded binary under
[air](https://github.com/air-verse/air), which rebuilds it on any change, and the
browser reloads itself over SSE (see `src/live.ts`). So the same binary serves
the app in dev and prod.

## Develop

From the repo root:

```
make serve                   # air rebuilds+restarts thingd on any change; open http://localhost:4319
make serve DIR=<path>        # ... against a specific data dir
```

The page reloads whenever thingd restarts with a new build, and the tree also
live-reloads over SSE when the data changes from any source (web, CLI, editor).

## Scripts

```
npm run build       # type-check then build to dist/
npm run typecheck   # tsc --noEmit
```

## Production

From the repo root, `make build` builds `dist/` and embeds it into `thingd` for a
single self-contained binary that serves the whole app. See
the root [README](../README.md#web-thingd).
