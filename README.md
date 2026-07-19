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

## License

MIT — see [LICENSE](LICENSE).
