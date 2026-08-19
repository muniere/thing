// The URL contract, in one place.
//
//   /                       the project picker
//   /<project>/             the board, nothing focused
//   /<project>/?ref=/<ref>  the board, focused on <ref>
//   /<project>/<ref>        the standalone screen for <ref>
//
// The board's path is canonically written with the trailing slash; a bare
// /<project> parses the same. Refs are slug paths ([a-z0-9-] joined by "/"), so
// they are URL-safe as-is. Filters and search live in the query string, next to
// the focused ref.
//
// The split is between what a node *is* and what the board is *doing*: a node's
// place in the tree is a path, and which node the board has focused is one more
// piece of that board's state, alongside its filters.

export type Route =
  | { kind: "picker" }
  | { kind: "board"; project: string; ref: string | null }
  | { kind: "node"; project: string; ref: string };

// parseLocation reads the current URL. Zero path segments is the picker; one is
// the board, whose focus comes from the query; two or more is a node, whose ref
// is everything after the project.
export function parseLocation(): Route {
  const segments = window.location.pathname.split("/").filter(Boolean);
  if (segments.length === 0) return { kind: "picker" };
  const [project, ...rest] = segments;
  if (rest.length === 0) {
    return { kind: "board", project, ref: refFromQuery(window.location.search) };
  }
  return { kind: "node", project, ref: rest.join("/") };
}

// refFromQuery reads the board's focus out of ?ref=/<ref>. The value carries a
// leading slash so it reads as a path within the project; a ref itself does not,
// so strip it back off.
export function refFromQuery(search: string): string | null {
  const ref = (new URLSearchParams(search).get("ref") ?? "").replace(/^\/+/, "");
  return ref || null;
}

// withoutRef returns the query minus the focus. It serves the one place that
// carries the URL's query through verbatim rather than rebuilding it (see
// useApp's queryString, which does so until the configured filter defaults have
// settled) and would otherwise hand boardHref a ref it is about to write again.
export function withoutRef(search: string): string {
  const p = new URLSearchParams(search);
  p.delete("ref");
  const s = p.toString();
  return s ? `?${s}` : "";
}

// boardHref builds a board URL: the project's path, the focused ref, and the
// filters. `query` is filtersToQuery()'s output ("" or "?…"), passed through so
// the filter encoding stays in filter.ts.
//
// The query is assembled by hand rather than through URLSearchParams, whose
// toString() percent-encodes the ref's slashes into an unreadable %2F chain. A
// slash is legal in a query string, so writing it plainly is both valid and what
// a person reading the URL expects.
export function boardHref(project: string, ref: string | null, query: string): string {
  const params = [...(ref ? [`ref=/${ref}`] : []), ...(query ? [query.slice(1)] : [])];
  return `/${project}/${params.length > 0 ? `?${params.join("&")}` : ""}`;
}

// nodeHref builds the standalone screen's URL. It carries no query: the screen
// is one node, and the board's filters mean nothing to it. An empty ref is the
// board itself, which is what that screen's logo links back to.
export function nodeHref(project: string, ref: string): string {
  return ref ? `/${project}/${ref}` : `/${project}/`;
}
